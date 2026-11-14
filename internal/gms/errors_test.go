package gms

import (
	"cmp"
	"errors"
	"testing"
)

const project2DemoNoJob = `  {
    "project": "project2",
    "environment": "DEMO",
    "error": "в ответе GitLab нет job.finishedAt"
  }`

const project2DevVersionErr = `  {
    "project": "project2",
    "environment": "DEV",
    "deployed_at": "2026-09-24T16:35:33+03:00",
    "commit": "1abfac43",
    "title": "Merge branch 'XXXXX-3488' into 'dev'",
    "error": "glab: 502 Bad Gateway"
  }`

const project1DevVersionErr = `  {
    "project": "project1",
    "environment": "DEV",
    "deployed_at": "2026-10-16T19:31:34+03:00",
    "commit": "f81c7da4",
    "title": "Merge branch 'XXXXX-3612' into 'dev'",
    "error": "glab: 502 Bad Gateway"
  }`

func TestRunShowsPairErrorInPlace(t *testing.T) {
	noVersionFile := fakeGitLab{
		"group/project2": {deploymentByEnv: testGitLab["group/project2"].deploymentByEnv},
		"group/project1": testGitLab["group/project1"],
	}
	tests := []struct {
		name   string
		config string
		gl     fakeGitLab
		faults glabFaults
		args   []string
		want   string
	}{
		{
			name:   "ошибка запроса текущих деплоев",
			faults: glabFaults{deploymentsErr: errors.New("glab: 500 Internal Server Error")},
			want: jsonReport(`  {
    "project": "project2",
    "environment": "DEV",
    "error": "glab: 500 Internal Server Error"
  }`, `  {
    "project": "project1",
    "environment": "DEV",
    "error": "glab: 500 Internal Server Error"
  }`),
		},
		{
			name:   "в ответе GitLab нет данных",
			faults: glabFaults{nullData: true},
			want: jsonReport(`  {
    "project": "project2",
    "environment": "DEV",
    "error": "в ответе GitLab нет data"
  }`, `  {
    "project": "project1",
    "environment": "DEV",
    "error": "в ответе GitLab нет data"
  }`),
		},
		{
			name: "проекта нет в GitLab",
			gl:   fakeGitLab{"group/project1": testGitLab["group/project1"]},
			want: jsonReport(`  {
    "project": "project2",
    "environment": "DEV",
    "error": "проект group/project2 не найден в GitLab или к нему нет доступа"
  }`, project1Dev),
		},
		{
			name:   "в деплое нет job",
			config: project2AllConfig,
			gl:     testGitLab.withDeployment("group/project2", "demo", "synthetic/last-deployment-no-job.json"),
			want:   jsonReport(project2Dev, project2DemoNoJob, project2Prod),
		},
		{
			name:   "ошибка пары среди выбранных окружений",
			config: project2AllConfig,
			gl:     testGitLab.withDeployment("group/project2", "demo", "synthetic/last-deployment-no-job.json"),
			args:   []string{"-e", "DEMO,PROD"},
			want:   jsonReport(project2DemoNoJob, project2Prod),
		},
		{
			name: "в деплое нет sha",
			gl:   testGitLab.withDeployment("group/project2", "dev", "synthetic/last-deployment-no-sha.json"),
			want: jsonReport(`  {
    "project": "project2",
    "environment": "DEV",
    "error": "в ответе GitLab нет commit.sha"
  }`, project1Dev),
		},
		{
			name:   "ошибка запроса версий",
			faults: glabFaults{versionsErr: errors.New("glab: 502 Bad Gateway")},
			want:   jsonReport(project2DevVersionErr, project1DevVersionErr),
		},
		{
			name: "на коммите деплоя нет application.version",
			gl:   noVersionFile,
			want: jsonReport(`  {
    "project": "project2",
    "environment": "DEV",
    "deployed_at": "2026-09-24T16:35:33+03:00",
    "commit": "1abfac43",
    "title": "Merge branch 'XXXXX-3488' into 'dev'",
    "error": "на коммите деплоя нет application.version"
  }`, project1Dev),
		},
		{
			name: "в application.version нет build_version",
			gl:   testGitLab.withVersion("group/project2", project2SHA, "synthetic/application.version-no-build-version"),
			want: jsonReport(`  {
    "project": "project2",
    "environment": "DEV",
    "deployed_at": "2026-09-24T16:35:33+03:00",
    "commit": "1abfac43",
    "title": "Merge branch 'XXXXX-3488' into 'dev'",
    "error": "в application.version нет build_version"
  }`, project1Dev),
		},
		{
			name: "в application.version пустой build_version",
			gl:   testGitLab.withVersion("group/project2", project2SHA, "synthetic/application.version-empty-build-version"),
			want: jsonReport(`  {
    "project": "project2",
    "environment": "DEV",
    "deployed_at": "2026-09-24T16:35:33+03:00",
    "commit": "1abfac43",
    "title": "Merge branch 'XXXXX-3488' into 'dev'",
    "error": "в application.version пустой build_version"
  }`, project1Dev),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gl := tt.gl
			if gl == nil {
				gl = testGitLab
			}
			assertRun(t, cmp.Or(tt.config, twoProjectsConfig), gl, tt.faults, tt.args, 1, tt.want, "")
		})
	}
}

func TestRunStopsWhenGlabUnusable(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		faults     glabFaults
		wantStderr string
	}{
		{
			name:   "glab не установлен",
			faults: glabFaults{notInstalled: true},
			wantStderr: "{\"error\":\"glab не найден\",\"hint\":\"установите его: https://gitlab.com/gitlab-org/cli, " +
				"затем выполните: glab auth login --hostname gitlab.example.com\"}\n",
		},
		{
			name:   "glab не залогинен в хост",
			faults: glabFaults{anonymousHosts: []string{host}},
			wantStderr: "{\"error\":\"glab не прошёл проверку входа в gitlab.example.com: GitLab не узнал пользователя glab\"," +
				"\"hint\":\"выполните: glab auth login --hostname gitlab.example.com\"}\n",
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
			faults: glabFaults{anonymousHosts: []string{"gitlab.example.org"}},
			wantStderr: "{\"error\":\"glab не прошёл проверку входа в gitlab.example.org: GitLab не узнал пользователя glab\"," +
				"\"hint\":\"выполните: glab auth login --hostname gitlab.example.org\"}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertRun(t, cmp.Or(tt.config, twoProjectsConfig), testGitLab, tt.faults, nil, 1, "", tt.wantStderr)
		})
	}
}
