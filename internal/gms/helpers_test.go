package gms

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const host = "gitlab.example.com"

// glabResponses сопоставляет аргументы вызова glab с файлом заготовки в testdata.
type glabResponses map[string]string

func deploymentsCall(project, env string) string {
	return "api --hostname " + host + " projects/" + project +
		"/deployments?environment=" + env + "&status=success&order_by=finished_at&sort=desc&per_page=1"
}

func versionCall(project, sha string) string {
	return "api --hostname " + host + " projects/" + project +
		"/repository/files/application.version/raw?ref=" + sha
}

// project2Responses — ответы GitLab для group/project2, снятые 2026-10-18.
var project2Responses = glabResponses{
	deploymentsCall("group%2Fproject2", "dev"):                                  "project2/deployments-dev.json",
	deploymentsCall("group%2Fproject2", "demo"):                                 "project2/deployments-demo.json",
	deploymentsCall("group%2Fproject2", "prod"):                                 "project2/deployments-prod.json",
	versionCall("group%2Fproject2", "1abfac438a0c3ad915d9525a4d2e4616e59007f8"): "project2/application.version-1abfac43",
}

// project1Responses — ответы GitLab для group/project1, снятые 2026-10-18. У окружений разные коммиты деплоя.
var project1Responses = glabResponses{
	deploymentsCall("group%2Fproject1", "dev"):                                  "project1/deployments-dev.json",
	deploymentsCall("group%2Fproject1", "demo"):                                 "project1/deployments-demo.json",
	deploymentsCall("group%2Fproject1", "prod"):                                 "project1/deployments-prod.json",
	versionCall("group%2Fproject1", "f81c7da43b3023ff00d8bfb0bc3e7740401776e7"): "project1/application.version-f81c7da4",
	versionCall("group%2Fproject1", "ad87e7d16ba3e32d5527166a81a47db41f3a56ba"): "project1/application.version-ad87e7d1",
	versionCall("group%2Fproject1", "eb9aad5ddd8d1e2912453039c2e57450d9ad070d"): "project1/application.version-eb9aad5d",
}

// glabErrors сопоставляет аргументы вызова glab с ошибкой, которую вернёт фейковый glab.
type glabErrors map[string]error

// authCall — проверка входа glab в хост h. Без ошибки в glabErrors фейковый glab её проходит.
func authCall(h string) string { return "auth status --hostname " + h }

func fakeGlab(t *testing.T, r glabResponses, errs glabErrors) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		key := strings.Join(args, " ")
		if err, ok := errs[key]; ok {
			return nil, err
		}
		if strings.HasPrefix(key, authCall("")) {
			return nil, nil
		}
		fixture, ok := r[key]
		if !ok {
			t.Errorf("неожиданный вызов glab: %s", key)
			return nil, errors.New("404 Not Found")
		}
		return os.ReadFile(filepath.Join("testdata", fixture))
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func moscow(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

// forbiddenGlab роняет тест при любом вызове glab.
func forbiddenGlab(t *testing.T) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		t.Errorf("неожиданный вызов glab: %s", strings.Join(args, " "))
		return nil, errors.New("glab запрещён в тесте")
	}
}

// runWithEnv запускает gms с переменными окружения env и glab и возвращает код выхода, stdout и stderr.
func runWithEnv(t *testing.T, env map[string]string, glab func(args ...string) ([]byte, error), args []string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, Deps{
		Glab:     glab,
		Getenv:   func(k string) string { return env[k] },
		HomeDir:  fakeHomeDir(env),
		Stdout:   &stdout,
		Stderr:   &stderr,
		Location: moscow(t),
	})
	return code, stdout.String(), stderr.String()
}

// fakeHomeDir ведёт себя как os.UserHomeDir в Windows: домашний каталог берётся из USERPROFILE.
func fakeHomeDir(env map[string]string) func() (string, error) {
	return func() (string, error) {
		if h := env["USERPROFILE"]; h != "" {
			return h, nil
		}
		return "", errors.New("USERPROFILE не задан")
	}
}

// configEnv возвращает переменные окружения gms с конфигом config во временном файле.
func configEnv(t *testing.T, config string) map[string]string {
	t.Helper()
	return map[string]string{"GMS_CONFIG": writeConfig(t, config)}
}

// assertRun запускает gms с конфигом config и фейковым glab и проверяет код выхода, stdout и stderr.
func assertRun(t *testing.T, config string, glab glabResponses, errs glabErrors, args []string,
	wantCode int, wantStdout, wantStderr string) {
	t.Helper()
	code, stdout, stderr := runWithEnv(t, configEnv(t, config), fakeGlab(t, glab, errs), args)
	assertResult(t, code, stdout, stderr, wantCode, wantStdout, wantStderr)
}

// assertResult сравнивает код выхода, stdout и stderr gms с ожидаемыми.
func assertResult(t *testing.T, code int, stdout, stderr string, wantCode int, wantStdout, wantStderr string) {
	t.Helper()
	if code != wantCode {
		t.Errorf("код выхода %d, want %d", code, wantCode)
	}
	if stdout != wantStdout {
		t.Errorf("stdout:\n%s\nwant:\n%s", stdout, wantStdout)
	}
	if stderr != wantStderr {
		t.Errorf("stderr:\n%s\nwant:\n%s", stderr, wantStderr)
	}
}

// assertOutput запускает gms с конфигом config и фейковым glab и проверяет успешный вывод want.
func assertOutput(t *testing.T, config string, glab glabResponses, args []string, want string) {
	t.Helper()
	assertRun(t, config, glab, nil, args, 0, want, "")
}

// mergeResponses объединяет заготовки ответов нескольких проектов.
func mergeResponses(rs ...glabResponses) glabResponses {
	all := glabResponses{}
	for _, r := range rs {
		for k, v := range r {
			all[k] = v
		}
	}
	return all
}

// twoProjectsConfig — project2 и project1 по одному окружению, project2 в файле первым.
const twoProjectsConfig = `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
  },
  "project1": {
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev"
  }
}`

// project2DevConfig — один проект project2 с одним окружением DEV на ветке dev.
const project2DevConfig = `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
  }
}`

// project2AllConfig — проект project2 с окружениями DEV, DEMO и PROD.
const project2AllConfig = `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project2/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project2/-/tree/main"
  }
}`

// allPairsConfig — project2 и project1 по три окружения.
const allPairsConfig = `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project2/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project2/-/tree/main"
  },
  "project1": {
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project1/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project1/-/tree/main"
  }
}`

// Блоки окружений по заготовкам project2Responses и project1Responses, без заголовка проекта.
const (
	project2DevBlock = `DEV:
2026-09-24 16:35:33 MSK
7.0.2
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
`
	project2DemoBlock = `DEMO:
2026-10-01 15:54:46 MSK
7.0.2
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
`
	project2ProdBlock = `PROD:
2026-10-05 11:38:34 MSK
7.0.2
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
`
	project1DevBlock = `DEV:
2026-10-16 19:31:34 MSK
81.0.2
f81c7da4 Merge branch 'XXXXX-3612' into 'dev'
`
	project1DemoBlock = `DEMO:
2026-10-16 19:02:37 MSK
75.3.0
ad87e7d1 Merge branch 'XXXXX-3602-main' into 'demo'
`
	project1ProdBlock = `PROD:
2026-10-16 09:21:17 MSK
75.3.0
eb9aad5d Merge branch 'XXXXX-3599' into 'demo'
`
)

const (
	envSep     = "\n---------------------------------------------------------------\n"
	projectSep = "\n\n===============================================================\n"
)

// Вывод проекта целиком: project2Dev и project1Dev — с одним окружением DEV, project2All и project1All — с DEV, DEMO и PROD.
const (
	project2Dev = "project2\n" + project2DevBlock
	project1Dev = "project1\n" + project1DevBlock
	project2All = "project2\n" + project2DevBlock + envSep + project2DemoBlock + envSep + project2ProdBlock
	project1All = "project1\n" + project1DevBlock + envSep + project1DemoBlock + envSep + project1ProdBlock
)
