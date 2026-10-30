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
	var envNames envNameList
	fs.Var(&codes, "p", "")
	fs.Var(&envNames, "e", "")
	fs.Bool("c", false, "") // -c уже обработан configRequested, здесь он нужен только для записи -c=false
	if err := fs.Parse(args); errors.Is(err, flag.ErrHelp) {
		fmt.Fprint(deps.Stdout, usage)
		return 0
	} else if err != nil {
		rep.usageError(err)
		return 2
	}
	if fs.NArg() > 0 {
		hintCodes, hintEnvNames := hintValues(args)
		rep.extraArgs(fs.Args(), hintCodes, hintEnvNames)
		return 2
	}
	if len(codes) == 0 && flagGiven(fs, "p") {
		rep.emptyCodes()
		return 2
	}
	if len(envNames) == 0 && flagGiven(fs, "e") {
		rep.emptyEnvNames()
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

	// Неизвестные коды и имена окружений сообщаются вместе, чтобы все опечатки исправлялись за один запуск.
	selected, unknownCodes := selectProjects(projects, codes)
	knownEnvs := knownEnvNames(projects)
	unknownEnvs := findUnknownEnvNames(knownEnvs, envNames)
	if len(unknownCodes) > 0 {
		rep.unknownCodes(projects, unknownCodes)
	}
	if len(unknownEnvs) > 0 {
		rep.unknownEnvNames(knownEnvs, unknownEnvs)
	}
	if len(unknownCodes) > 0 || len(unknownEnvs) > 0 {
		return 2
	}
	selected = selectEnvironments(selected, envNames)
	if len(selected) == 0 {
		// Все коды и имена известны, значит, пусто только при -p вместе с -e.
		rep.noSelectedEnvs(codes, envNames)
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
		name, value, hasValue, isFlag := flagArg(a)
		if !isFlag || name != "c" {
			return false
		}
		on, err := strconv.ParseBool(value)
		return !hasValue || err == nil && on
	})
}

// flagArg разбирает аргумент a как флаг в любой записи, которую принимает flag: -name, --name, -name=value,
// --name=value. isFlag сообщает, записан ли a как флаг.
func flagArg(a string) (name, value string, hasValue, isFlag bool) {
	a, isFlag = strings.CutPrefix(a, "-")
	if !isFlag {
		return "", "", false, false
	}
	name, value, hasValue = strings.Cut(strings.TrimPrefix(a, "-"), "=")
	return name, value, hasValue, true
}

// codeList — значение флага -p: коды проектов через запятую. Повторный флаг дописывает коды,
// пустые элементы пропускаются, повторы кода схлопываются по первому вхождению.
type codeList []string

func (c *codeList) String() string { return strings.Join(*c, ",") }

func (c *codeList) Set(s string) error {
	*c = appendValues(*c, s, func(a, b string) bool { return a == b })
	return nil
}

// envNameList — значение флага -e: имена окружений через запятую по правилам codeList,
// но повторы имени сравниваются без учёта регистра.
type envNameList []string

func (e *envNameList) String() string { return strings.Join(*e, ",") }

func (e *envNameList) Set(s string) error {
	*e = appendValues(*e, s, sameEnvName)
	return nil
}

// appendValues дописывает к list значения из s через запятую: пробелы вокруг значений обрезаются, пустые значения
// пропускаются, значения, которые same считает повтором уже записанных, не дописываются.
func appendValues(list []string, s string, same func(a, b string) bool) []string {
	for v := range strings.SplitSeq(s, ",") {
		v = strings.TrimSpace(v)
		if v == "" || slices.ContainsFunc(list, func(w string) bool { return same(v, w) }) {
			continue
		}
		list = append(list, v)
	}
	return list
}

// hintValues собирает из args значения -p и -e для подсказки так, будто они перечислены через пробел: каждое
// позиционное слово относится к ближайшему флагу -p или -e слева, а слово без такого флага — к -p. Разбирать
// приходится самим: flag не разбирает аргументы после первого позиционного.
func hintValues(args []string) (codes codeList, envNames envNameList) {
	var current flag.Value = &codes
	for i := 0; i < len(args); i++ {
		name, value, hasValue, isFlag := flagArg(args[i])
		if !isFlag {
			current.Set(args[i])
			continue
		}
		switch name {
		case "p":
			current = &codes
		case "e":
			current = &envNames
		default:
			continue // другие флаги в подсказку не идут
		}
		// Следующий флаг не берётся в значение, иначе подсказку нельзя будет запустить.
		if !hasValue && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			value = args[i]
		}
		current.Set(value)
	}
	return codes, envNames
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

// selectEnvironments оставляет у проектов окружения с именами names без учёта регистра в порядке names,
// а без names — все окружения. Проекты, у которых не осталось окружений, не возвращаются.
func selectEnvironments(projects []Project, names []string) []Project {
	if len(names) == 0 {
		return projects
	}
	var selected []Project
	for _, p := range projects {
		var envs []Environment
		for _, name := range names {
			for _, env := range p.Environments {
				if sameEnvName(env.Name, name) {
					envs = append(envs, env)
				}
			}
		}
		if len(envs) > 0 {
			p.Environments = envs
			selected = append(selected, p)
		}
	}
	return selected
}

// sameEnvName сообщает, называют ли a и b одно окружение: имена окружений сравниваются без учёта регистра.
func sameEnvName(a, b string) bool { return strings.EqualFold(a, b) }

// containsEnvName сообщает, есть ли среди names имя того же окружения, что name.
func containsEnvName(names []string, name string) bool {
	return slices.ContainsFunc(names, func(n string) bool { return sameEnvName(n, name) })
}

// knownEnvNames возвращает имена окружений всех проектов в порядке первого появления, без повторов.
func knownEnvNames(projects []Project) []string {
	var known []string
	for _, p := range projects {
		for _, env := range p.Environments {
			if !containsEnvName(known, env.Name) {
				known = append(known, env.Name)
			}
		}
	}
	return known
}

// findUnknownEnvNames возвращает имена из names, которых нет среди известных имён known.
func findUnknownEnvNames(known, names []string) []string {
	var unknown []string
	for _, name := range names {
		if !containsEnvName(known, name) {
			unknown = append(unknown, name)
		}
	}
	return unknown
}
