package gms

import (
	"cmp"
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

// Заготовки в testdata/synthetic не сняты с GitLab: deployments-no-deployable и deployments-no-sha —
// это project2/deployments-dev.json без deployable и без sha. application.version-no-build-version — одна строка
// version=7.0.2 без build_version, application.version-empty-build-version — одна строка build_version= без значения.
func TestRunShowsPairErrorInPlace(t *testing.T) {
	tests := []struct {
		name   string
		config string        // по умолчанию twoProjectsConfig
		glab   glabResponses // подменяет заготовки project2Responses и project1Responses
		errs   glabErrors
		want   string
	}{
		{
			name: "ошибка текущего деплоя",
			errs: glabErrors{
				deploymentsCall("group%2Fproject2", "dev"): errors.New("glab: 404 Project Not Found (HTTP 404)"),
			},
			want: `project2
DEV:
ошибка: glab: 404 Project Not Found (HTTP 404)
` + projectSep + project1Dev,
		},
		{
			name:   "ошибка текущего деплоя между окружениями проекта",
			config: project2AllConfig,
			errs: glabErrors{
				deploymentsCall("group%2Fproject2", "demo"): errors.New("glab: 500 Internal Server Error (HTTP 500)"),
			},
			want: "project2\n" + project2DevBlock + envSep + `DEMO:
ошибка: glab: 500 Internal Server Error (HTTP 500)
` + envSep + project2ProdBlock,
		},
		{
			name: "в деплое нет deployable",
			glab: glabResponses{
				deploymentsCall("group%2Fproject2", "dev"): "synthetic/deployments-no-deployable.json",
			},
			want: `project2
DEV:
ошибка: в ответе Deployments API нет deployable.finished_at
` + projectSep + project1Dev,
		},
		{
			name: "в деплое нет sha",
			glab: glabResponses{
				deploymentsCall("group%2Fproject2", "dev"): "synthetic/deployments-no-sha.json",
			},
			want: `project2
DEV:
ошибка: в ответе Deployments API нет sha
` + projectSep + project1Dev,
		},
		{
			name: "ошибка версии",
			errs: glabErrors{
				versionCall("group%2Fproject2", "1abfac438a0c3ad915d9525a4d2e4616e59007f8"): errors.New("glab: 404 File Not Found (HTTP 404)"),
			},
			want: `project2
DEV:
2026-09-24 16:35:33 MSK
версия: ошибка (glab: 404 File Not Found (HTTP 404))
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
` + projectSep + project1Dev,
		},
		{
			name: "в application.version нет build_version",
			glab: glabResponses{
				versionCall("group%2Fproject2", "1abfac438a0c3ad915d9525a4d2e4616e59007f8"): "synthetic/application.version-no-build-version",
			},
			want: `project2
DEV:
2026-09-24 16:35:33 MSK
версия: ошибка (в application.version нет build_version)
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
` + projectSep + project1Dev,
		},
		{
			name: "в application.version пустой build_version",
			glab: glabResponses{
				versionCall("group%2Fproject2", "1abfac438a0c3ad915d9525a4d2e4616e59007f8"): "synthetic/application.version-empty-build-version",
			},
			want: `project2
DEV:
2026-09-24 16:35:33 MSK
версия: ошибка (в application.version пустой build_version)
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
` + projectSep + project1Dev,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := cmp.Or(tt.config, twoProjectsConfig)
			glab := mergeResponses(project2Responses, project1Responses, tt.glab)
			assertRun(t, config, glab, tt.errs, nil, 1, tt.want, "")
		})
	}
}

// TestRunStopsWhenGlabUnusable проверяет, что без рабочего glab gms выходит до запросов к API:
// в заготовках нет ни одного ответа API, и любой такой запрос уронит тест.
func TestRunStopsWhenGlabUnusable(t *testing.T) {
	tests := []struct {
		name       string
		config     string // по умолчанию twoProjectsConfig
		errs       glabErrors
		wantStderr string
	}{
		{
			name: "glab не установлен",
			errs: glabErrors{authCall(host): fmt.Errorf("exec: \"glab\": %w", exec.ErrNotFound)},
			wantStderr: "gms: glab не найден, установите его: https://gitlab.com/gitlab-org/cli\n" +
				"затем выполните: glab auth login --hostname gitlab.example.com\n",
		},
		{
			name: "glab не залогинен в хост",
			errs: glabErrors{authCall(host): errors.New("glab: X gitlab.example.com has not been authenticated with glab")},
			wantStderr: "gms: glab не прошёл проверку входа в gitlab.example.com: " +
				"glab: X gitlab.example.com has not been authenticated with glab\n" +
				"выполните: glab auth login --hostname gitlab.example.com\n",
		},
		{
			name: "glab не залогинен во второй хост конфига",
			config: `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
  },
  "project1": {
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev",
    "DEMO": "https://gitlab.example.org/group/project1/-/tree/demo"
  }
}`,
			errs: glabErrors{authCall("gitlab.example.org"): errors.New("glab: X gitlab.example.org has not been authenticated with glab")},
			wantStderr: "gms: glab не прошёл проверку входа в gitlab.example.org: " +
				"glab: X gitlab.example.org has not been authenticated with glab\n" +
				"выполните: glab auth login --hostname gitlab.example.org\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRun(t, cmp.Or(tt.config, twoProjectsConfig), nil, tt.errs, nil, 1, "", tt.wantStderr)
		})
	}
}
