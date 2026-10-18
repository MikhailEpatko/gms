package gms

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

const dateLayout = "2006-01-02 15:04:05 MST"

var (
	envRule     = strings.Repeat("-", 63)
	projectRule = strings.Repeat("=", 63)
)

// formatter печатает результаты в текстовом формате gms.
type formatter struct {
	w   io.Writer
	loc *time.Location
}

func (f formatter) projectHeader(code string) {
	fmt.Fprintln(f.w, code)
}

func (f formatter) projectSeparator() {
	fmt.Fprintf(f.w, "\n\n%s\n", projectRule)
}

func (f formatter) envSeparator() {
	fmt.Fprintf(f.w, "\n%s\n", envRule)
}

// pair печатает пару проект+окружение: текущий деплой, «нет деплоев» или ошибку на месте пары.
// При ошибке версии вместо версии стоит её ошибка, дата и коммит печатаются.
func (f formatter) pair(env Environment, r pairResult) {
	switch {
	case r.err != nil:
		fmt.Fprintf(f.w, "%s:\nошибка: %s\n", env.Name, r.err)
	case r.deployment == nil:
		fmt.Fprintf(f.w, "%s:\nнет деплоев\n", env.Name)
	default:
		d := r.deployment
		version := r.version
		if r.versionErr != nil {
			version = "версия: ошибка (" + r.versionErr.Error() + ")"
		}
		fmt.Fprintf(f.w, "%s:\n%s%s\n%s\n%s %s\n",
			env.Name, d.FinishedAt.In(f.loc).Format(dateLayout), d.mark(env.Branch), version, d.ShortID, d.Title)
	}
}

// mark возвращает пометку ref деплоя для строки даты: пусто, если деплой сделан из ветки окружения branch.
// Деплой из MR приходит с ref refs/merge-requests/<N>/head, а из merged results pipeline — с .../merge.
func (d Deployment) mark(branch string) string {
	if mr, ok := strings.CutPrefix(d.Ref, "refs/merge-requests/"); ok {
		if n, kind, ok := strings.Cut(mr, "/"); ok && (kind == "head" || kind == "merge") {
			return " (MR !" + n + ")"
		}
	}
	if d.Ref == branch {
		return ""
	}
	return " (ветка: " + d.Ref + ")"
}

const usage = `использование: gms [-p код,код] [-c]

Показывает текущий деплой каждого окружения проектов из конфига:
дату деплоя, версию приложения и коммит деплоя.

  -p код,код  только перечисленные проекты, в порядке аргументов
  -c          напечатать путь к конфигу и выйти
  -h          показать эту справку

Конфиг: ~/.config/gms/config.json или путь из переменной GMS_CONFIG.
`

// exampleConfig — обезличенный пример конфига для сообщения о его отсутствии.
const exampleConfig = `{
  "project1": {
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project1/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project1/-/tree/main"
  },
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev",
    "PROD": "https://gitlab.example.com/group/project2/-/tree/main"
  }
}
`

// reporter печатает в stderr ошибки, после которых gms завершается.
type reporter struct {
	w io.Writer
}

func (r reporter) failure(err error) {
	fmt.Fprintln(r.w, "gms:", err)
}

// usageError сообщает об ошибке разбора флагов и печатает справку.
func (r reporter) usageError(err error) {
	fmt.Fprintf(r.w, "gms: %v\n%s", err, usage)
}

// extraArgs сообщает о позиционных аргументах extra и подсказывает запись кодов codes через -p.
func (r reporter) extraArgs(extra, codes []string) {
	fmt.Fprintf(r.w, "gms: лишние аргументы: %s\nкоды проектов перечисляются через запятую: gms -p %s\n",
		strings.Join(extra, " "), strings.Join(codes, ","))
}

func (r reporter) emptyCodes() {
	fmt.Fprintln(r.w, "gms: в -p не указан ни один код проекта, пример: -p project1,project3")
}

func (r reporter) missingConfig(path string) {
	fmt.Fprintf(r.w, "gms: нет файла конфига, создайте %s\nпример:\n%s", path, exampleConfig)
}

// unknownCodes сообщает о кодах unknown, которых нет в конфиге, и перечисляет коды проектов projects.
func (r reporter) unknownCodes(projects []Project, unknown []string) {
	known := make([]string, len(projects))
	for i, p := range projects {
		known[i] = p.Code
	}
	label := "неизвестный код проекта"
	if len(unknown) > 1 {
		label = "неизвестные коды проектов"
	}
	fmt.Fprintf(r.w, "gms: %s: %s\nизвестные коды: %s\n", label, strings.Join(unknown, ", "), strings.Join(known, ", "))
}

// glabUnusable сообщает, что glab не прошёл проверку входа в host с ошибкой err, и подсказывает, как это исправить.
func (r reporter) glabUnusable(host string, err error) {
	if errors.Is(err, exec.ErrNotFound) {
		fmt.Fprintf(r.w, "gms: glab не найден, установите его: https://gitlab.com/gitlab-org/cli\n"+
			"затем выполните: glab auth login --hostname %s\n", host)
		return
	}
	// Причину glab показываем как есть: кроме отсутствия входа это бывает и сбой сети или прокси.
	fmt.Fprintf(r.w, "gms: glab не прошёл проверку входа в %s: %v\n"+
		"выполните: glab auth login --hostname %s\n", host, err, host)
}
