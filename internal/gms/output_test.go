package gms

import (
	"testing"
)

func TestRunShowsCurrentDeploymentOfProject(t *testing.T) {
	tests := []struct {
		name   string
		config string
		glab   glabResponses
		args   []string
		want   string
	}{
		{
			name:   "одно окружение",
			config: project2DevConfig,
			glab:   project2Responses,
			args:   []string{"-p", "project2"},
			want:   project2Dev,
		},
		{
			name:   "окружения разделены линией",
			config: project2AllConfig,
			glab:   project2Responses,
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
			glab: project1Responses,
			args: []string{"-p", "project1"},
			want: "project1\n" + project1ProdBlock + envSep + project1DevBlock + envSep + project1DemoBlock,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOutput(t, tt.config, tt.glab, tt.args, tt.want)
		})
	}
}

// Заготовки в testdata/synthetic не сняты с GitLab: deployments-mr, deployments-mr-merge и deployments-ci —
// это project2/deployments-dev.json с подменённым ref, deployments-empty — пустой ответ Deployments API.
func TestRunMarksDeploymentRef(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		want    string
	}{
		{
			name:    "деплой из MR",
			fixture: "synthetic/deployments-mr.json",
			want: `project2
DEV:
2026-09-24 16:35:33 MSK (MR !23)
7.0.2
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
`,
		},
		{
			name:    "деплой из merged results pipeline MR",
			fixture: "synthetic/deployments-mr-merge.json",
			want: `project2
DEV:
2026-09-24 16:35:33 MSK (MR !23)
7.0.2
1abfac43 Merge branch 'XXXXX-3488' into 'dev'
`,
		},
		{
			name:    "деплой из чужой ветки",
			fixture: "synthetic/deployments-ci.json",
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
			glab := mergeResponses(project2Responses, glabResponses{
				deploymentsCall("group%2Fproject2", "dev"): tt.fixture,
			})
			assertOutput(t, project2DevConfig, glab, []string{"-p", "project2"}, tt.want)
		})
	}
}

func TestRunShowsNoDeployments(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   string
	}{
		{
			name:   "единственное окружение",
			config: project2DevConfig,
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
			want: "project2\n" + project2DemoBlock + envSep + `DEV:
нет деплоев
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Запроса версии DEV в ответах нет: если он будет выполнен, fakeGlab уронит тест.
			glab := glabResponses{
				deploymentsCall("group%2Fproject2", "demo"):                                 "project2/deployments-demo.json",
				deploymentsCall("group%2Fproject2", "dev"):                                  "synthetic/deployments-empty.json",
				versionCall("group%2Fproject2", "1abfac438a0c3ad915d9525a4d2e4616e59007f8"): "project2/application.version-1abfac43",
			}
			assertOutput(t, tt.config, glab, []string{"-p", "project2"}, tt.want)
		})
	}
}
