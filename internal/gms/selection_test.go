package gms

import (
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
			want: project2Dev + projectSep + project1Dev,
		},
		{
			name: "-c=false не мешает выводу",
			args: []string{"-c=false"},
			want: project2Dev + projectSep + project1Dev,
		},
		{
			name: "с -p проекты в порядке аргументов",
			args: []string{"-p", "project1,project2"},
			want: project1Dev + projectSep + project2Dev,
		},
		{
			name: "пробелы вокруг кодов обрезаются",
			args: []string{"-p", " project1 , project2 "},
			want: project1Dev + projectSep + project2Dev,
		},
		{
			name: "пустой элемент в середине пропускается",
			args: []string{"-p", "project1,,project2"},
			want: project1Dev + projectSep + project2Dev,
		},
		{
			name: "пустой элемент в конце пропускается",
			args: []string{"-p", "project1,"},
			want: project1Dev,
		},
		{
			name: "повторный флаг накапливает коды",
			args: []string{"-p", "project1", "-p", "project2"},
			want: project1Dev + projectSep + project2Dev,
		},
		{
			name: "повторы кода схлопываются по первому вхождению",
			args: []string{"-p", "project1,project2,project1", "-p", "project2"},
			want: project1Dev + projectSep + project2Dev,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOutput(t, twoProjectsConfig, testGitLab, tt.args, tt.want)
		})
	}
}
