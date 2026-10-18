package gms

import (
	"cmp"
	"errors"
	"testing"
)

// Заготовки в testdata/synthetic не сняты с GitLab: last-deployment-no-job и last-deployment-no-sha —
// это project2/last-deployment-dev.json с job: null и без commit.sha. application.version-no-build-version —
// одна строка version=7.0.2 без build_version, application.version-empty-build-version — одна строка build_version=
// без значения.
func TestRunShowsPairErrorInPlace(t *testing.T) {
	// На коммите деплоя project2 нет application.version.
	noVersionFile := fakeGitLab{
		"group/project2": {deployments: testGitLab["group/project2"].deployments},
		"group/project1": testGitLab["group/project1"],
	}
	tests := []struct {
		name   string
		config string     // по умолчанию twoProjectsConfig
		gl     fakeGitLab // по умолчанию testGitLab
		faults glabFaults
		want   string
	}{
		{
			name:   "ошибка запроса текущих деплоев",
			faults: glabFaults{deployments: errors.New("glab: 500 Internal Server Error")},
			want: `project2
DEV:
ошибка: glab: 500 Internal Server Error
` + projectSep + `project1
DEV:
ошибка: glab: 500 Internal Server Error
`,
		},
		{
			name:   "в ответе GitLab нет данных",
			faults: glabFaults{nullData: true},
			want: `project2
DEV:
ошибка: в ответе GitLab нет data
` + projectSep + `project1
DEV:
ошибка: в ответе GitLab нет data
`,
		},
		{
			name: "проекта нет в GitLab",
			gl:   fakeGitLab{"group/project1": testGitLab["group/project1"]},
			want: `project2
DEV:
ошибка: проект group/project2 не найден в GitLab или к нему нет доступа
` + projectSep + project1Dev,
		},
		{
			name:   "в деплое нет job",
			config: project2AllConfig,
			gl:     testGitLab.withDeployment("group/project2", "demo", "synthetic/last-deployment-no-job.json"),
			want: "project2\n" + project2DevBlock + envSep + `DEMO:
ошибка: в ответе GitLab нет job.finishedAt
` + envSep + project2ProdBlock,
		},
		{
			name: "в деплое нет sha",
			gl:   testGitLab.withDeployment("group/project2", "dev", "synthetic/last-deployment-no-sha.json"),
			want: `project2
DEV:
ошибка: в ответе GitLab нет commit.sha
` + projectSep + project1Dev,
		},
		{
			name:   "ошибка запроса версий",
			faults: glabFaults{versions: errors.New("glab: 502 Bad Gateway")},
			want: `project2
DEV:
2026-09-24 16:35:33 MSK
версия: ошибка (glab: 502 Bad Gateway)
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
` + projectSep + `project1
DEV:
2026-10-16 19:31:34 MSK
версия: ошибка (glab: 502 Bad Gateway)
f81c7da4 Merge branch 'XXXXX-3612' into 'dev'
`,
		},
		{
			name: "на коммите деплоя нет application.version",
			gl:   noVersionFile,
			want: `project2
DEV:
2026-09-24 16:35:33 MSK
версия: ошибка (на коммите деплоя нет application.version)
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
` + projectSep + project1Dev,
		},
		{
			name: "в application.version нет build_version",
			gl:   testGitLab.withVersion("group/project2", project2SHA, "synthetic/application.version-no-build-version"),
			want: `project2
DEV:
2026-09-24 16:35:33 MSK
версия: ошибка (в application.version нет build_version)
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
` + projectSep + project1Dev,
		},
		{
			name: "в application.version пустой build_version",
			gl:   testGitLab.withVersion("group/project2", project2SHA, "synthetic/application.version-empty-build-version"),
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
			gl := tt.gl
			if gl == nil {
				gl = testGitLab
			}
			assertRun(t, cmp.Or(tt.config, twoProjectsConfig), gl, tt.faults, nil, 1, tt.want, "")
		})
	}
}

// TestRunStopsWhenGlabUnusable проверяет, что без рабочего glab gms ничего не выводит в stdout и выходит с подсказкой.
func TestRunStopsWhenGlabUnusable(t *testing.T) {
	tests := []struct {
		name       string
		config     string // по умолчанию twoProjectsConfig
		faults     glabFaults
		wantStderr string
	}{
		{
			name:   "glab не установлен",
			faults: glabFaults{notInstalled: true},
			wantStderr: "gms: glab не найден, установите его: https://gitlab.com/gitlab-org/cli\n" +
				"затем выполните: glab auth login --hostname gitlab.example.com\n",
		},
		{
			name:   "glab не залогинен в хост",
			faults: glabFaults{anonymous: []string{host}},
			wantStderr: "gms: glab не прошёл проверку входа в gitlab.example.com: GitLab не узнал пользователя glab\n" +
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
			faults: glabFaults{anonymous: []string{"gitlab.example.org"}},
			wantStderr: "gms: glab не прошёл проверку входа в gitlab.example.org: GitLab не узнал пользователя glab\n" +
				"выполните: glab auth login --hostname gitlab.example.org\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRun(t, cmp.Or(tt.config, twoProjectsConfig), testGitLab, tt.faults, nil, 1, "", tt.wantStderr)
		})
	}
}
