// Package gms показывает текущий деплой каждого окружения проектов .
package gms

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Deps — внешние зависимости Run. Единственный seam для тестов.
type Deps struct {
	// Glab запускает glab: аргументы → stdout или ошибка. Если glab не найден, ошибка оборачивает exec.ErrNotFound.
	Glab   func(args ...string) ([]byte, error)
	Getenv func(string) string
	// HomeDir возвращает домашний каталог, когда HOME не задан, как в Windows (os.UserHomeDir).
	HomeDir  func() (string, error)
	Stdout   io.Writer
	Stderr   io.Writer
	Location *time.Location // часовой пояс вывода даты деплоя
}

// Run выполняет gms с аргументами командной строки args (без имени программы) и возвращает код выхода.
func Run(args []string, deps Deps) int {
	rep := reporter{w: deps.Stderr}
	if configRequested(args) {
		path, err := configPath(deps.Getenv, deps.HomeDir)
		if err != nil {
			rep.failure(err)
			return 1
		}
		fmt.Fprintln(deps.Stdout, path)
		return 0
	}

	// Справку и ошибки разбора печатаем сами: у flag нет русской справки и префикса gms.
	fs := flag.NewFlagSet("gms", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var codes codeList
	fs.Var(&codes, "p", "")
	fs.Bool("c", false, "") // -c уже обработан configRequested, здесь он нужен только для записи -c=false
	if err := fs.Parse(args); errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(deps.Stdout, usage)
		return 0
	} else if err != nil {
		rep.usageError(err)
		return 2
	}
	if fs.NArg() > 0 {
		// В подсказку идут коды: флаги после первого позиционного аргумента flag не разбирает.
		hint := slices.DeleteFunc(slices.Concat(codes, fs.Args()), func(a string) bool { return strings.HasPrefix(a, "-") })
		rep.extraArgs(fs.Args(), hint)
		return 2
	}
	if len(codes) == 0 && flagGiven(fs, "p") {
		rep.emptyCodes()
		return 2
	}

	path, err := configPath(deps.Getenv, deps.HomeDir)
	if err != nil {
		rep.failure(err)
		return 1
	}
	projects, err := loadConfig(path)
	if errors.Is(err, os.ErrNotExist) {
		rep.missingConfig(path)
		return 1
	}
	if err != nil {
		rep.failure(err)
		return 1
	}

	selected, unknown := selectProjects(projects, codes)
	if len(unknown) > 0 {
		rep.unknownCodes(projects, unknown)
		return 2
	}
	var envs []Environment // все пары в порядке вывода
	for _, p := range selected {
		envs = append(envs, p.Environments...)
	}
	results, err := gitlab{glab: deps.Glab}.pairs(envs)
	if unusable, ok := errors.AsType[*unusableError](err); ok {
		rep.glabUnusable(unusable.host, unusable.err)
		return 1
	}

	f := formatter{w: deps.Stdout, loc: deps.Location}
	failed := false // хотя бы одна пара завершилась ошибкой
	k := 0          // индекс пары в results
	for i, p := range selected {
		if i > 0 {
			f.projectSeparator()
		}
		f.projectHeader(p.Code)
		for j, env := range p.Environments {
			if j > 0 {
				f.envSeparator()
			}
			f.pair(env, results[k])
			failed = failed || results[k].failed()
			k++
		}
	}
	if failed {
		return 1
	}
	return 0
}

// configRequested сообщает, есть ли в args флаг -c в любой записи, которую принимает flag: -c, --c, -c=<истина>.
// Ищем его до разбора флагов, потому что -c отменяет все остальные аргументы, а разбор останавливается раньше:
// на неизвестном флаге или на первом позиционном аргументе.
func configRequested(args []string) bool {
	return slices.ContainsFunc(args, func(a string) bool {
		a, ok := strings.CutPrefix(a, "-")
		if !ok {
			return false
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(a, "-"), "=")
		if name != "c" {
			return false
		}
		on, err := strconv.ParseBool(value)
		return !hasValue || err == nil && on
	})
}

// codeList — значение флага -p: коды проектов через запятую. Повторный флаг дописывает коды,
// пустые элементы пропускаются, повторы кода схлопываются по первому вхождению.
type codeList []string

func (c *codeList) String() string { return strings.Join(*c, ",") }

func (c *codeList) Set(s string) error {
	for _, code := range strings.Split(s, ",") {
		code = strings.TrimSpace(code)
		if code == "" || slices.Contains(*c, code) {
			continue
		}
		*c = append(*c, code)
	}
	return nil
}

// flagGiven сообщает, был ли флаг name в командной строке.
func flagGiven(fs *flag.FlagSet, name string) bool {
	given := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			given = true
		}
	})
	return given
}

// selectProjects возвращает проекты в порядке codes, а без codes — все проекты в порядке конфига.
// unknown — коды из codes, которых нет в конфиге.
func selectProjects(projects []Project, codes []string) (selected []Project, unknown []string) {
	if len(codes) == 0 {
		return projects, nil
	}
	for _, code := range codes {
		i := slices.IndexFunc(projects, func(p Project) bool { return p.Code == code })
		if i < 0 {
			unknown = append(unknown, code)
			continue
		}
		selected = append(selected, projects[i])
	}
	return selected, unknown
}
