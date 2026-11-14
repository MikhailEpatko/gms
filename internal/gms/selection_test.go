package gms

import (
	"cmp"
	"slices"
	"sync"
	"testing"
)

func TestRunSelectsProjects(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "без флагов все проекты в порядке конфига",
			args: nil,
			want: jsonReport(project2Dev, project1Dev),
		},
		{
			name: "-c=false не мешает выводу",
			args: []string{"-c=false"},
			want: jsonReport(project2Dev, project1Dev),
		},
		{
			name: "с -p проекты в порядке аргументов",
			args: []string{"-p", "project1,project2"},
			want: jsonReport(project1Dev, project2Dev),
		},
		{
			name: "пробелы вокруг кодов обрезаются",
			args: []string{"-p", " project1 , project2 "},
			want: jsonReport(project1Dev, project2Dev),
		},
		{
			name: "пустой элемент в середине пропускается",
			args: []string{"-p", "project1,,project2"},
			want: jsonReport(project1Dev, project2Dev),
		},
		{
			name: "пустой элемент в конце пропускается",
			args: []string{"-p", "project1,"},
			want: jsonReport(project1Dev),
		},
		{
			name: "повторный флаг накапливает коды",
			args: []string{"-p", "project1", "-p", "project2"},
			want: jsonReport(project1Dev, project2Dev),
		},
		{
			name: "повторы кода схлопываются по первому вхождению",
			args: []string{"-p", "project1,project2,project1", "-p", "project2"},
			want: jsonReport(project1Dev, project2Dev),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOutput(t, twoProjectsConfig, testGitLab, tt.args, tt.want)
		})
	}
}

func TestRunSelectsEnvironments(t *testing.T) {
	tests := []struct {
		name   string
		config string
		args   []string
		want   string
	}{
		{
			name: "окружения всех проектов",
			args: []string{"-e", "DEV,DEMO"},
			want: jsonReport(project2Dev, project2Demo, project1Dev, project1Demo),
		},
		{
			name: "одно окружение одного проекта",
			args: []string{"-e", "PROD", "-p", "project1"},
			want: jsonReport(project1Prod),
		},
		{
			name: "окружения в порядке аргументов",
			args: []string{"-e", "PROD,DEV", "-p", "project1"},
			want: jsonReport(project1Prod, project1Dev),
		},
		{
			name: "проекты в порядке -p",
			args: []string{"-e", "DEV", "-p", "project1,project2"},
			want: jsonReport(project1Dev, project2Dev),
		},
		{
			name: "имя без учёта регистра, в выводе как в конфиге",
			args: []string{"-e", "prod,Demo", "-p", "project1"},
			want: jsonReport(project1Prod, project1Demo),
		},
		{
			name: "пробелы вокруг имён обрезаются",
			args: []string{"-e", " PROD , DEV ", "-p", "project1"},
			want: jsonReport(project1Prod, project1Dev),
		},
		{
			name: "пустые элементы пропускаются",
			args: []string{"-e", "PROD,,DEV,", "-p", "project1"},
			want: jsonReport(project1Prod, project1Dev),
		},
		{
			name: "повторный флаг накапливает имена",
			args: []string{"-e", "PROD", "-e", "DEV", "-p", "project1"},
			want: jsonReport(project1Prod, project1Dev),
		},
		{
			name: "повторы схлопываются по первому вхождению без учёта регистра",
			args: []string{"-e", "dev,DEMO,DEV", "-e", "demo", "-p", "project1"},
			want: jsonReport(project1Dev, project1Demo),
		},
		{
			name: "запись -e=",
			args: []string{"-e=PROD", "-p", "project1"},
			want: jsonReport(project1Prod),
		},
		{
			name: "запись --e",
			args: []string{"--e", "PROD", "-p", "project1"},
			want: jsonReport(project1Prod),
		},
		{
			name:   "проект без окружения не выводится",
			config: project2WithoutDemoConfig,
			args:   []string{"-e", "DEMO"},
			want:   jsonReport(project1Demo),
		},
		{
			name:   "проект без окружения не выводится и при -p",
			config: project2WithoutDemoConfig,
			args:   []string{"-e", "DEMO", "-p", "project2,project1"},
			want:   jsonReport(project1Demo),
		},
		{
			name:   "у проекта выводятся только его окружения из -e",
			config: project2WithoutDemoConfig,
			args:   []string{"-e", "DEMO,PROD"},
			want:   jsonReport(project2Prod, project1Demo, project1Prod),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOutput(t, cmp.Or(tt.config, allPairsConfig), testGitLab, tt.args, tt.want)
		})
	}
}

func TestRunRequestsOnlySelectedPairs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		want          string
		wantRequested []string
	}{
		{
			name:          "окружение всех проектов",
			args:          []string{"-e", "PROD"},
			want:          jsonReport(project2Prod, project1Prod),
			wantRequested: []string{"group/project1 prod", "group/project2 prod"},
		},
		{
			name:          "окружения одного проекта",
			args:          []string{"-e", "demo,prod", "-p", "project1"},
			want:          jsonReport(project1Demo, project1Prod),
			wantRequested: []string{"group/project1 demo", "group/project1 prod"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mu sync.Mutex
			var requested []string
			glab := fakeGlab(t, testGitLab, glabFaults{})
			recording := func(args ...string) ([]byte, error) {
				query, _ := graphqlQuery(args)
				mu.Lock()
				requested = append(requested, deploymentPairs(query)...)
				mu.Unlock()
				return glab(args...)
			}
			code, stdout, stderr := runWithEnv(t, configEnv(t, allPairsConfig), recording, tt.args)
			assertResult(t, code, stdout, stderr, 0, tt.want, "")
			slices.Sort(requested)
			if !slices.Equal(requested, tt.wantRequested) {
				t.Errorf("запрошены пары %q, want %q", requested, tt.wantRequested)
			}
		})
	}
}
