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

const (
	anyHost       = ""
	noDeployments = ""
)

type fakeProject struct {
	host             string
	deploymentByEnv  map[string]string
	versionFileBySHA map[string]string
}

type fakeGitLab map[string]fakeProject

var testGitLab = fakeGitLab{
	"group/project2": {
		deploymentByEnv: map[string]string{
			"dev":  "project2/last-deployment-dev.json",
			"demo": "project2/last-deployment-demo.json",
			"prod": "project2/last-deployment-prod.json",
		},
		versionFileBySHA: map[string]string{
			"1abfac438a0c3ad915d9525a4d2e4616e59007f8": "project2/application.version-1abfac43",
		},
	},
	"group/project1": {
		deploymentByEnv: map[string]string{
			"dev":  "project1/last-deployment-dev.json",
			"demo": "project1/last-deployment-demo.json",
			"prod": "project1/last-deployment-prod.json",
		},
		versionFileBySHA: map[string]string{
			"f81c7da43b3023ff00d8bfb0bc3e7740401776e7": "project1/application.version-f81c7da4",
			"ad87e7d16ba3e32d5527166a81a47db41f3a56ba": "project1/application.version-ad87e7d1",
			"eb9aad5ddd8d1e2912453039c2e57450d9ad070d": "project1/application.version-eb9aad5d",
		},
	},
}

const project2SHA = "1abfac438a0c3ad915d9525a4d2e4616e59007f8"

func (gl fakeGitLab) withDeployment(path, env, fixture string) fakeGitLab {
	gl = maps.Clone(gl)
	p := gl[path]
	p.deploymentByEnv = maps.Clone(p.deploymentByEnv)
	p.deploymentByEnv[env] = fixture
	gl[path] = p
	return gl
}

func (gl fakeGitLab) withVersion(path, sha, fixture string) fakeGitLab {
	gl = maps.Clone(gl)
	p := gl[path]
	p.versionFileBySHA = maps.Clone(p.versionFileBySHA)
	p.versionFileBySHA[sha] = fixture
	gl[path] = p
	return gl
}

func onHost(p fakeProject, h string) fakeProject {
	p.host = h
	return p
}

type glabFaults struct {
	deploymentsErr error
	versionsErr    error
	anonymousHosts []string
	notInstalled   bool
	nullData       bool
}

var graphqlField = regexp.MustCompile(
	`(\w+):\s*(project\(fullPath|environment\(name|repository\s*\{\s*blobs\(ref):\s*"([^"]*)"`)

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
		case strings.Contains(query, "environment(") && faults.deploymentsErr != nil:
			return nil, faults.deploymentsErr
		case strings.Contains(query, "blobs(") && faults.versionsErr != nil:
			return nil, faults.versionsErr
		}
		if faults.nullData {
			return []byte(`{"data": null}`), nil
		}
		data := map[string]any{}
		if strings.Contains(query, "currentUser") {
			data["currentUser"] = map[string]any{"username": "tester"}
			if slices.Contains(faults.anonymousHosts, args[2]) {
				data["currentUser"] = nil
			}
		}
		var currentProject map[string]any
		var fp fakeProject
		for _, m := range graphqlField.FindAllStringSubmatch(query, -1) {
			alias, field, arg := m[1], m[2], m[3]
			switch {
			case strings.HasPrefix(field, "project"):
				var found bool
				if fp, found = gl[arg]; found && (fp.host == anyHost || fp.host == args[2]) {
					currentProject = map[string]any{}
					data[alias] = currentProject
				} else {
					currentProject = nil
					data[alias] = nil
				}
			case currentProject == nil:
			case strings.HasPrefix(field, "environment"):
				fixture, found := fp.deploymentByEnv[arg]
				switch {
				case !found:
					currentProject[alias] = nil
				case fixture == noDeployments:
					currentProject[alias] = map[string]any{"lastDeployment": nil}
				default:
					currentProject[alias] = map[string]any{"lastDeployment": json.RawMessage(readFixture(t, fixture))}
				}
			default:
				nodes := []any{}
				if fixture, found := fp.versionFileBySHA[arg]; found {
					nodes = append(nodes, map[string]any{"rawBlob": string(readFixture(t, fixture))})
				}
				currentProject[alias] = map[string]any{"blobs": map[string]any{"nodes": nodes}}
			}
		}
		return json.Marshal(map[string]any{"data": data})
	}
}

func deploymentPairs(query string) []string {
	var pairs []string
	var project string
	for _, m := range graphqlField.FindAllStringSubmatch(query, -1) {
		switch field, arg := m[2], m[3]; {
		case strings.HasPrefix(field, "project"):
			project = arg
		case strings.HasPrefix(field, "environment"):
			pairs = append(pairs, project+" "+arg)
		}
	}
	return pairs
}

func graphqlQuery(args []string) (string, bool) {
	if len(args) != 6 || args[0] != "api" || args[1] != "--hostname" || args[3] != "graphql" || args[4] != "-f" {
		return "", false
	}
	return strings.CutPrefix(args[5], "query=")
}

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

func forbiddenGlab(t *testing.T) func(args ...string) ([]byte, error) {
	return func(args ...string) ([]byte, error) {
		t.Errorf("неожиданный вызов glab: %s", strings.Join(args, " "))
		return nil, errors.New("glab запрещён в тесте")
	}
}

func runWithEnv(t *testing.T, env map[string]string, glab func(args ...string) ([]byte, error), args []string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, Deps{
		Glab:     glab,
		Getenv:   func(k string) string { return env[k] },
		HomeDir:  fakeWindowsHomeDir(env),
		Stdout:   &stdout,
		Stderr:   &stderr,
		Location: moscow(t),
	})
	return code, stdout.String(), stderr.String()
}

func fakeWindowsHomeDir(env map[string]string) func() (string, error) {
	return func() (string, error) {
		if h := env["USERPROFILE"]; h != "" {
			return h, nil
		}
		return "", errors.New("USERPROFILE не задан")
	}
}

func configEnv(t *testing.T, config string) map[string]string {
	t.Helper()
	return map[string]string{"GMS_CONFIG": writeConfig(t, config)}
}

func assertRun(t *testing.T, config string, gl fakeGitLab, faults glabFaults, args []string,
	wantCode int, wantStdout, wantStderr string) {
	t.Helper()
	code, stdout, stderr := runWithEnv(t, configEnv(t, config), fakeGlab(t, gl, faults), args)
	assertResult(t, code, stdout, stderr, wantCode, wantStdout, wantStderr)
}

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

func assertOutput(t *testing.T, config string, gl fakeGitLab, args []string, want string) {
	t.Helper()
	assertRun(t, config, gl, glabFaults{}, args, 0, want, "")
}

const twoProjectsConfig = `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
  },
  "project1": {
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev"
  }
}`

const project2DevConfig = `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
  }
}`

const project2AllConfig = `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project2/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project2/-/tree/main"
  }
}`

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

const project2WithoutDemoConfig = `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev",
    "PROD": "https://gitlab.example.com/group/project2/-/tree/main"
  },
  "project1": {
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project1/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project1/-/tree/main"
  }
}`

func jsonReport(pairs ...string) string {
	doc := "[\n" + strings.Join(pairs, ",\n") + "\n]\n"
	if !json.Valid([]byte(doc)) {
		panic("невалидный JSON в ожиданиях теста:\n" + doc)
	}
	return doc
}

const project2Dev = `  {
    "project": "project2",
    "environment": "DEV",
    "deployed_at": "2026-09-24T16:35:33+03:00",
    "version": "7.0.2",
    "commit": "1abfac43",
    "title": "Merge branch 'XXXXX-3488' into 'dev'"
  }`

const project2Demo = `  {
    "project": "project2",
    "environment": "DEMO",
    "deployed_at": "2026-10-01T15:54:46+03:00",
    "version": "7.0.2",
    "commit": "1abfac43",
    "title": "Merge branch 'XXXXX-3488' into 'dev'"
  }`

const project2Prod = `  {
    "project": "project2",
    "environment": "PROD",
    "deployed_at": "2026-10-05T11:38:34+03:00",
    "version": "7.0.2",
    "commit": "1abfac43",
    "title": "Merge branch 'XXXXX-3488' into 'dev'"
  }`

const project1Dev = `  {
    "project": "project1",
    "environment": "DEV",
    "deployed_at": "2026-10-16T19:31:34+03:00",
    "version": "81.0.2",
    "commit": "f81c7da4",
    "title": "Merge branch 'XXXXX-3612' into 'dev'"
  }`

const project1Demo = `  {
    "project": "project1",
    "environment": "DEMO",
    "deployed_at": "2026-10-16T19:02:37+03:00",
    "version": "75.3.0",
    "commit": "ad87e7d1",
    "title": "Merge branch 'XXXXX-3602-main' into 'demo'"
  }`

const project1Prod = `  {
    "project": "project1",
    "environment": "PROD",
    "deployed_at": "2026-10-16T09:21:17+03:00",
    "version": "75.3.0",
    "commit": "eb9aad5d",
    "title": "Merge branch 'XXXXX-3599' into 'demo'"
  }`

const (
	project2All = "[\n" + project2Dev + ",\n" + project2Demo + ",\n" + project2Prod + "\n]\n"
	project1All = "[\n" + project1Dev + ",\n" + project1Demo + ",\n" + project1Prod + "\n]\n"
)
