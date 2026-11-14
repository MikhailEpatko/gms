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
			want:   jsonReport(project2Dev),
		},
		{
			name:   "все окружения проекта",
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
			want: jsonReport(project1Prod, project1Dev, project1Demo),
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
			want: jsonReport(project2Dev, project1Dev),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOutput(t, tt.config, tt.gl, tt.args, tt.want)
		})
	}
}

const project2DevNoDeploys = `  {
    "project": "project2",
    "environment": "DEV",
    "error": "Нет деплоев"
  }`

func TestRunKeepsTitleCharactersUnescaped(t *testing.T) {
	gl := testGitLab.withDeployment("group/project2", "dev", "synthetic/last-deployment-title-html.json")
	assertOutput(t, project2DevConfig, gl, []string{"-p", "project2"}, `[
  {
    "project": "project2",
    "environment": "DEV",
    "deployed_at": "2026-09-24T16:35:33+03:00",
    "version": "7.0.2",
    "commit": "1abfac43",
    "title": "Fix <login> & retry"
  }
]
`)
}

func TestRunShowsNoDeployments(t *testing.T) {
	noDevDeployments := testGitLab.withDeployment("group/project2", "dev", noDeployments)
	noDevEnvironment := fakeGitLab{"group/project2": {
		deploymentByEnv:  map[string]string{"demo": "project2/last-deployment-demo.json"},
		versionFileBySHA: testGitLab["group/project2"].versionFileBySHA,
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
			want:   jsonReport(project2DevNoDeploys),
		},
		{
			name: "несколько окружений, одно без деплоев",
			config: `{
  "project2": {
    "DEMO": "https://gitlab.example.com/group/project2/-/tree/demo",
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
  }
}`,
			gl:   noDevDeployments,
			want: jsonReport(project2Demo, project2DevNoDeploys),
		},
		{
			name:   "окружения нет в GitLab",
			config: project2DevConfig,
			gl:     noDevEnvironment,
			want:   jsonReport(project2DevNoDeploys),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOutput(t, tt.config, tt.gl, []string{"-p", "project2"}, tt.want)
		})
	}
}
