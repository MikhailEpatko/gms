package gms

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
)

const host = "gitlab.example.com"

// fakeProject — проект в фейковом GitLab.
type fakeProject struct {
	// host — хост GitLab проекта: на запрос к другому хосту фейк отвечает, что проекта нет. "" — любой хост.
	host string
	// deployments: окружение → заготовка узла lastDeployment, "" — окружение есть, но успешных деплоев нет.
	// Окружения без ключа в GitLab нет.
	deployments map[string]string
	// versions: sha коммита → заготовка application.version. Для sha без ключа файла на коммите нет.
	versions map[string]string
}

// fakeGitLab — проекты фейкового GitLab по пути проекта. Проекта без ключа нет.
type fakeGitLab map[string]fakeProject

// testGitLab — group/project2 и group/project1, ответы GraphQL сняты 2026-10-18.
// У окружений project2 один коммит деплоя, у окружений project1 разные.
var testGitLab = fakeGitLab{
	"group/project2": {
		deployments: map[string]string{
			"dev":  "project2/last-deployment-dev.json",
			"demo": "project2/last-deployment-demo.json",
			"prod": "project2/last-deployment-prod.json",
		},
		versions: map[string]string{
			"1abfac438a0c3ad915d9525a4d2e4616e59007f8": "project2/application.version-1abfac43",
		},
	},
	"group/project1": {
		deployments: map[string]string{
			"dev":  "project1/last-deployment-dev.json",
			"demo": "project1/last-deployment-demo.json",
			"prod": "project1/last-deployment-prod.json",
		},
		versions: map[string]string{
			"f81c7da43b3023ff00d8bfb0bc3e7740401776e7": "project1/application.version-f81c7da4",
			"ad87e7d16ba3e32d5527166a81a47db41f3a56ba": "project1/application.version-ad87e7d1",
			"eb9aad5ddd8d1e2912453039c2e57450d9ad070d": "project1/application.version-eb9aad5d",
		},
	},
}

const project2SHA = "1abfac438a0c3ad915d9525a4d2e4616e59007f8"

// withDeployment возвращает копию gl, в которой у окружения env проекта path заготовка lastDeployment fixture.
func (gl fakeGitLab) withDeployment(path, env, fixture string) fakeGitLab {
	gl = maps.Clone(gl)
	p := gl[path]
	p.deployments = maps.Clone(p.deployments)
	p.deployments[env] = fixture
	gl[path] = p
	return gl
}

// withVersion возвращает копию gl, в которой у проекта path на коммите sha заготовка application.version fixture.
func (gl fakeGitLab) withVersion(path, sha, fixture string) fakeGitLab {
	gl = maps.Clone(gl)
	p := gl[path]
	p.versions = maps.Clone(p.versions)
	p.versions[sha] = fixture
	gl[path] = p
	return gl
}

// onHost возвращает проект p, который есть только на хосте h.
func onHost(p fakeProject, h string) fakeProject {
	p.host = h
	return p
}

// glabFaults — сбои фейкового glab.
type glabFaults struct {
	deployments error    // ошибка каждого запроса текущих деплоев
	versions    error    // ошибка каждого запроса версий
	anonymous   []string // хосты, в которые glab не вошёл: GitLab отвечает без текущего пользователя
	// notInstalled: glab не установлен, любой вызов завершается ошибкой с exec.ErrNotFound.
	notInstalled bool
	// nullData: GitLab отвечает на каждый запрос {"data": null}, а glab завершается без ошибки.
	nullData bool
}

// graphqlField — поле запроса с алиасом, которое понимает фейковый GitLab: project, environment или blobs.
var graphqlField = regexp.MustCompile(
	`(\w+):\s*(project\(fullPath|environment\(name|repository\s*\{\s*blobs\(ref):\s*"([^"]*)"`)

// fakeGlab отвечает на glab api graphql данными gl, как ответил бы GitLab: несуществующий проект
// или окружение — null, коммит без файла — пустой список blobs. Любой другой вызов glab роняет тест.
func fakeGlab(t *testing.T, gl fakeGitLab, faults glabFaults) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		if faults.notInstalled {
			return nil, fmt.Errorf("exec: \"glab\": %w", exec.ErrNotFound)
		}
		query, ok := graphqlQuery(args)
		if !ok {
			t.Errorf("неожиданный вызов glab: %s", strings.Join(args, " "))
			return nil, errors.New("неожиданный вызов glab")
		}
		switch {
		case strings.Contains(query, "environment(") && faults.deployments != nil:
			return nil, faults.deployments
		case strings.Contains(query, "blobs(") && faults.versions != nil:
			return nil, faults.versions
		}
		if faults.nullData {
			return []byte(`{"data": null}`), nil
		}
		data := map[string]any{}
		if strings.Contains(query, "currentUser") {
			data["currentUser"] = map[string]any{"username": "tester"}
			if slices.Contains(faults.anonymous, args[2]) {
				data["currentUser"] = nil
			}
		}
		var project map[string]any // поля текущего проекта запроса, nil — проекта нет
		var fp fakeProject
		for _, m := range graphqlField.FindAllStringSubmatch(query, -1) {
			alias, field, arg := m[1], m[2], m[3]
			switch {
			case strings.HasPrefix(field, "project"):
				var found bool
				if fp, found = gl[arg]; found && (fp.host == "" || fp.host == args[2]) {
					project = map[string]any{}
					data[alias] = project
				} else {
					project = nil
					data[alias] = nil
				}
			case project == nil:
			case strings.HasPrefix(field, "environment"):
				fixture, found := fp.deployments[arg]
				switch {
				case !found:
					project[alias] = nil
				case fixture == "":
					project[alias] = map[string]any{"lastDeployment": nil}
				default:
					project[alias] = map[string]any{"lastDeployment": json.RawMessage(readFixture(t, fixture))}
				}
			default:
				nodes := []any{}
				if fixture, found := fp.versions[arg]; found {
					nodes = append(nodes, map[string]any{"rawBlob": string(readFixture(t, fixture))})
				}
				project[alias] = map[string]any{"blobs": map[string]any{"nodes": nodes}}
			}
		}
		return json.Marshal(map[string]any{"data": data})
	}
}

// graphqlQuery возвращает запрос из вызова glab api --hostname <хост> graphql -f query=<запрос>.
func graphqlQuery(args []string) (string, bool) {
	if len(args) != 6 || args[0] != "api" || args[1] != "--hostname" || args[3] != "graphql" || args[4] != "-f" {
		return "", false
	}
	return strings.CutPrefix(args[5], "query=")
}

// readFixture читает заготовку из testdata. Её вызывает glab из горутин gms, поэтому ошибка не останавливает тест.
func readFixture(t *testing.T, name string) []byte {
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Error(err)
	}
	return b
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
func assertRun(t *testing.T, config string, gl fakeGitLab, faults glabFaults, args []string,
	wantCode int, wantStdout, wantStderr string) {
	t.Helper()
	code, stdout, stderr := runWithEnv(t, configEnv(t, config), fakeGlab(t, gl, faults), args)
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
func assertOutput(t *testing.T, config string, gl fakeGitLab, args []string, want string) {
	t.Helper()
	assertRun(t, config, gl, glabFaults{}, args, 0, want, "")
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
