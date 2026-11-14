package gms

import (
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"strings"
	"time"
)

type pairReport struct {
	Project     string     `json:"project"`
	Environment string     `json:"environment"`
	DeployedAt  *time.Time `json:"deployed_at,omitempty"`
	Version     string     `json:"version,omitempty"`
	Commit      string     `json:"commit,omitempty"`
	Title       string     `json:"title,omitempty"`
	Error       string     `json:"error,omitempty"`
}

func newPairReport(code, name string, r pairResult, loc *time.Location) pairReport {
	report := pairReport{Project: code, Environment: name}
	switch {
	case r.deploymentErr != nil:
		report.Error = r.deploymentErr.Error()
	case r.deployment == nil:
		report.Error = "Нет деплоев"
	default:
		d := r.deployment
		finishedAt := d.FinishedAt.In(loc)
		report.DeployedAt = &finishedAt
		report.Commit = d.ShortID
		report.Title = d.Title
		if r.versionErr != nil {
			report.Error = r.versionErr.Error()
		} else {
			report.Version = r.version
		}
	}
	return report
}

func jsonEncoder(w io.Writer) *json.Encoder {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc
}

func writeReport(w io.Writer, report []pairReport) {
	enc := jsonEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(report)
}

const usage = `использование: gms [-p код,код] [-e имя,имя] [-c]

Показывает текущий деплой каждого окружения проектов из конфига:
дату деплоя, версию приложения и коммит деплоя.
Вывод — JSON: отчёт в stdout, ошибки и подсказки в stderr.

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

type message struct {
	Error   string   `json:"error,omitempty"`
	Hint    string   `json:"hint,omitempty"`
	Usage   string   `json:"usage,omitempty"`
	Config  string   `json:"config,omitempty"`
	Path    string   `json:"path,omitempty"`
	Example string   `json:"example,omitempty"`
	Known   []string `json:"known,omitempty"`
}

func writeMessage(w io.Writer, m message) {
	_ = jsonEncoder(w).Encode(m)
}

type reporter struct {
	w io.Writer
}

func (r reporter) failure(err error) {
	writeMessage(r.w, message{Error: err.Error()})
}

func (r reporter) usageError(err error) {
	writeMessage(r.w, message{Error: err.Error(), Usage: usage})
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
	writeMessage(r.w, message{
		Error: "лишние аргументы: " + strings.Join(extra, " "),
		Hint:  what + " перечисляются через запятую: " + cmd,
	})
}

func (r reporter) emptyCodes() {
	writeMessage(r.w, message{
		Error: "в -p не указан ни один код проекта",
		Hint:  "пример: -p project1,project3",
	})
}

func (r reporter) emptyEnvNames() {
	writeMessage(r.w, message{
		Error: "в -e не указано ни одно окружение",
		Hint:  "пример: -e DEV,DEMO",
	})
}

func (r reporter) missingConfig(path string) {
	writeMessage(r.w, message{Error: "нет файла конфига", Path: path, Example: exampleConfig})
}

func (r reporter) unknownCodes(projects []Project, unknown []string) {
	known := make([]string, len(projects))
	for i, p := range projects {
		known[i] = p.Code
	}
	label := plural(len(unknown), "неизвестный код проекта", "неизвестные коды проектов")
	writeMessage(r.w, message{Error: label + ": " + strings.Join(unknown, ", "), Known: known})
}

func (r reporter) unknownEnvNames(known, unknown []string) {
	label := plural(len(unknown), "неизвестное окружение", "неизвестные окружения")
	writeMessage(r.w, message{Error: label + ": " + strings.Join(unknown, ", "), Known: known})
}

func (r reporter) noSelectedEnvs(codes, names []string) {
	writeMessage(r.w, message{Error: "у " + plural(len(codes), "проекта", "проектов") + " " + strings.Join(codes, ", ") +
		" нет " + plural(len(names), "окружения", "окружений") + " " + strings.Join(names, ", ")})
}

func plural(n int, one, many string) string {
	if n > 1 {
		return many
	}
	return one
}

func (r reporter) glabUnusable(host string, err error) {
	if errors.Is(err, exec.ErrNotFound) {
		writeMessage(r.w, message{
			Error: "glab не найден",
			Hint:  "установите его: https://gitlab.com/gitlab-org/cli, затем выполните: glab auth login --hostname " + host,
		})
		return
	}
	writeMessage(r.w, message{
		Error: "glab не прошёл проверку входа в " + host + ": " + err.Error(),
		Hint:  "выполните: glab auth login --hostname " + host,
	})
}
