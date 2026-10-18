package gms

import (
	"testing"
)

func TestRunShowsCurrentDeploymentOfProject(t *testing.T) {
	tests := []struct {
		name   string
		config string
		gl     fakeGitLab
		args   []string
		want   string
	}{
		{
			name:   "одно окружение",
			config: project2DevConfig,
			gl:     testGitLab,
			args:   []string{"-p", "project2"},
			want:   project2Dev,
		},
		{
			name:   "окружения разделены линией",
			config: project2AllConfig,
			gl:     testGitLab,
			args:   []string{"-p", "project2"},
			want:   project2All,
		},
		{
			name: "окружения в порядке конфига, версия на коммите деплоя",
			config: `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
  },
  "project1": {
    "PROD": "https://gitlab.example.com/group/project1/-/tree/main",
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project1/-/tree/demo"
  }
}`,
			gl:   testGitLab,
			args: []string{"-p", "project1"},
			want: "project1\n" + project1ProdBlock + envSep + project1DevBlock + envSep + project1DemoBlock,
		},
		{
			name: "проекты на разных хостах",
			config: `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
  },
  "project1": {
    "DEV": "https://gitlab.example.org/group/project1/-/tree/dev"
  }
}`,
			gl: fakeGitLab{
				"group/project2": onHost(testGitLab["group/project2"], "gitlab.example.com"),
				"group/project1": onHost(testGitLab["group/project1"], "gitlab.example.org"),
			},
			want: project2Dev + projectSep + project1Dev,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOutput(t, tt.config, tt.gl, tt.args, tt.want)
		})
	}
}

// Заготовки в testdata/synthetic не сняты с GitLab: last-deployment-mr, last-deployment-mr-merge и last-deployment-ci —
// это project2/last-deployment-dev.json с подменённым ref.
func TestRunMarksDeploymentRef(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    string
	}{
		{
			name:    "деплой из MR",
			fixture: "synthetic/last-deployment-mr.json",
			want: `project2
DEV:
2026-09-24 16:35:33 MSK (MR !23)
7.0.2
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
`,
		},
		{
			name:    "деплой из merged results pipeline MR",
			fixture: "synthetic/last-deployment-mr-merge.json",
			want: `project2
DEV:
2026-09-24 16:35:33 MSK (MR !23)
7.0.2
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
`,
		},
		{
			name:    "деплой из чужой ветки",
			fixture: "synthetic/last-deployment-ci.json",
			want: `project2
DEV:
2026-09-24 16:35:33 MSK (ветка: ci)
7.0.2
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gl := testGitLab.withDeployment("group/project2", "dev", tt.fixture)
			assertOutput(t, project2DevConfig, gl, []string{"-p", "project2"}, tt.want)
		})
	}
}

func TestRunShowsNoDeployments(t *testing.T) {
	// У DEV нет успешных деплоев.
	noDevDeployments := testGitLab.withDeployment("group/project2", "dev", "")
	// Окружения DEV в GitLab нет.
	noDevEnvironment := fakeGitLab{"group/project2": {
		deployments: map[string]string{"demo": "project2/last-deployment-demo.json"},
		versions:    testGitLab["group/project2"].versions,
	}}
	tests := []struct {
		name   string
		config string
		gl     fakeGitLab
		want   string
	}{
		{
			name:   "единственное окружение",
			config: project2DevConfig,
			gl:     noDevDeployments,
			want: `project2
DEV:
нет деплоев
`,
		},
		{
			name: "после разделителя окружений",
			config: `{
  "project2": {
    "DEMO": "https://gitlab.example.com/group/project2/-/tree/demo",
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
  }
}`,
			gl: noDevDeployments,
			want: "project2\n" + project2DemoBlock + envSep + `DEV:
нет деплоев
`,
		},
		{
			name:   "окружения нет в GitLab",
			config: project2DevConfig,
			gl:     noDevEnvironment,
			want: `project2
DEV:
нет деплоев
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOutput(t, tt.config, tt.gl, []string{"-p", "project2"}, tt.want)
		})
	}
}
