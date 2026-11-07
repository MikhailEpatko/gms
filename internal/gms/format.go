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

type formatter struct {
	w   io.Writer
	loc *time.Location
}

func (f formatter) pair(code string, env Environment, r pairResult) {
	switch {
	case r.deploymentErr != nil:
		_, _ = fmt.Fprintf(f.w, "%s %s ошибка: %s\n", code, env.Name, r.deploymentErr)
	case r.deployment == nil:
		_, _ = fmt.Fprintf(f.w, "%s %s нет деплоев\n", code, env.Name)
	default:
		d := r.deployment
		version := r.version
		if r.versionErr != nil {
			version = "версия: ошибка (" + r.versionErr.Error() + ")"
		}
		_, _ = fmt.Fprintf(f.w, "%s %s %s %s %s %s%s\n",
			code, env.Name, d.FinishedAt.In(f.loc).Format(dateLayout), version, d.ShortID, d.Title, d.mark(env.Branch))
	}
}

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

const usage = `использование: gms [-p код,код] [-e имя,имя] [-c]

Показывает текущий деплой каждого окружения проектов из конфига:
дату деплоя, версию приложения и коммит деплоя.

  -p код,код  только перечисленные проекты, в порядке аргументов
  -e имя,имя  только перечисленные окружения, в порядке аргументов, регистр не важен
  -c          напечатать путь к конфигу и выйти
  -h          показать эту справку

Конфиг: ~/.config/gms/config.json или путь из переменной GMS_CONFIG.
`

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

type reporter struct {
	w io.Writer
}

func (r reporter) failure(err error) {
	_, _ = fmt.Fprintln(r.w, "gms:", err)
}

func (r reporter) usageError(err error) {
	_, _ = fmt.Fprintf(r.w, "gms: %v\n%s", err, usage)
}

func (r reporter) extraArgs(extra, codes, envNames []string) {
	what, cmd := "коды проектов", "gms"
	if len(codes) > 0 {
		cmd += " -p " + strings.Join(codes, ",")
	}
	if len(envNames) > 0 {
		cmd += " -e " + strings.Join(envNames, ",")
		what = "окружения"
		if len(codes) > 0 {
			what = "коды проектов и окружения"
		}
	}
	_, _ = fmt.Fprintf(r.w, "gms: лишние аргументы: %s\n%s перечисляются через запятую: %s\n", strings.Join(extra, " "), what, cmd)
}

func (r reporter) emptyCodes() {
	_, _ = fmt.Fprintln(r.w, "gms: в -p не указан ни один код проекта, пример: -p project1,project3")
}

func (r reporter) emptyEnvNames() {
	_, _ = fmt.Fprintln(r.w, "gms: в -e не указано ни одно окружение, пример: -e DEV,DEMO")
}

func (r reporter) missingConfig(path string) {
	_, _ = fmt.Fprintf(r.w, "gms: нет файла конфига, создайте %s\nпример:\n%s", path, exampleConfig)
}

func (r reporter) unknownCodes(projects []Project, unknown []string) {
	known := make([]string, len(projects))
	for i, p := range projects {
		known[i] = p.Code
	}
	label := plural(len(unknown), "неизвестный код проекта", "неизвестные коды проектов")
	_, _ = fmt.Fprintf(r.w, "gms: %s: %s\nизвестные коды: %s\n", label, strings.Join(unknown, ", "), strings.Join(known, ", "))
}

func (r reporter) unknownEnvNames(known, unknown []string) {
	label := plural(len(unknown), "неизвестное окружение", "неизвестные окружения")
	_, _ = fmt.Fprintf(r.w, "gms: %s: %s\nизвестные окружения: %s\n", label, strings.Join(unknown, ", "), strings.Join(known, ", "))
}

func (r reporter) noSelectedEnvs(codes, names []string) {
	_, _ = fmt.Fprintf(r.w, "gms: у %s %s нет %s %s\n", plural(len(codes), "проекта", "проектов"), strings.Join(codes, ", "),
		plural(len(names), "окружения", "окружений"), strings.Join(names, ", "))
}

func plural(n int, one, many string) string {
	if n > 1 {
		return many
	}
	return one
}

func (r reporter) glabUnusable(host string, err error) {
	if errors.Is(err, exec.ErrNotFound) {
		_, _ = fmt.Fprintf(r.w, "gms: glab не найден, установите его: https://gitlab.com/gitlab-org/cli\n"+
			"затем выполните: glab auth login --hostname %s\n", host)
		return
	}
	_, _ = fmt.Fprintf(r.w, "gms: glab не прошёл проверку входа в %s: %v\n"+
		"выполните: glab auth login --hostname %s\n", host, err, host)
}
