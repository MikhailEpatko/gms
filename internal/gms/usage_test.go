package gms

import (
	"testing"
)

func TestRunPrintsConfigPath(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		args []string
		want string
	}{
		{
			name: "по умолчанию в домашнем каталоге",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"-c"},
			want: "/home/dev/.config/gms/config.json\n",
		},
		{
			name: "без HOME в домашнем каталоге ОС",
			env:  map[string]string{"USERPROFILE": "/users/dev"},
			args: []string{"-c"},
			want: "/users/dev/.config/gms/config.json\n",
		},
		{
			name: "HOME важнее домашнего каталога ОС",
			env:  map[string]string{"HOME": "/home/dev", "USERPROFILE": "/users/dev"},
			args: []string{"-c"},
			want: "/home/dev/.config/gms/config.json\n",
		},
		{
			name: "из GMS_CONFIG",
			env:  map[string]string{"HOME": "/home/dev", "GMS_CONFIG": "/etc/gms.json"},
			args: []string{"-c"},
			want: "/etc/gms.json\n",
		},
		{
			name: "с двумя дефисами",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"--c"},
			want: "/home/dev/.config/gms/config.json\n",
		},
		{
			name: "со значением true",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"-c=true"},
			want: "/home/dev/.config/gms/config.json\n",
		},
		{
			name: "другие флаги и аргументы игнорируются",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"-p", "foo", "-c", "-p", ",", "project1"},
			want: "/home/dev/.config/gms/config.json\n",
		},
		{
			name: "после позиционного аргумента и неизвестного флага",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"-x", "-p", "project1", "project3", "-c"},
			want: "/home/dev/.config/gms/config.json\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Конфига по этим путям нет: -c его не читает.
			code, stdout, stderr := runWithEnv(t, tt.env, forbiddenGlab(t), tt.args)
			assertResult(t, code, stdout, stderr, 0, tt.want, "")
		})
	}
}

const wantUsage = `использование: gms [-p код,код] [-c]

Показывает текущий деплой каждого окружения проектов из конфига:
дату деплоя, версию приложения и коммит деплоя.

  -p код,код  только перечисленные проекты, в порядке аргументов
  -c          напечатать путь к конфигу и выйти
  -h          показать эту справку

Конфиг: ~/.config/gms/config.json или путь из переменной GMS_CONFIG.
`

func TestRunShowsHelp(t *testing.T) {
	code, stdout, stderr := runWithEnv(t, configEnv(t, twoProjectsConfig), forbiddenGlab(t), []string{"-h"})
	assertResult(t, code, stdout, stderr, 0, wantUsage, "")
}

// TestRunRejectsUsageErrors проверяет, что ошибки использования дают код выхода 2 до любых вызовов glab.
func TestRunRejectsUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "неизвестный флаг",
			args:       []string{"-x"},
			wantStderr: "gms: flag provided but not defined: -x\n" + wantUsage,
		},
		{
			name:       "-c с некорректным значением",
			args:       []string{"-c=yes"},
			wantStderr: "gms: invalid boolean value \"yes\" for -c: parse error\n" + wantUsage,
		},
		{
			name:       "коды через пробел после -p",
			args:       []string{"-p", "project1", "project3"},
			wantStderr: "gms: лишние аргументы: project3\nкоды проектов перечисляются через запятую: gms -p project1,project3\n",
		},
		{
			name:       "код без -p",
			args:       []string{"project1"},
			wantStderr: "gms: лишние аргументы: project1\nкоды проектов перечисляются через запятую: gms -p project1\n",
		},
		{
			name:       "код без -p перед -p",
			args:       []string{"project1", "-p", "project3"},
			wantStderr: "gms: лишние аргументы: project1 -p project3\nкоды проектов перечисляются через запятую: gms -p project1,project3\n",
		},
		{
			name:       "-p с пустой строкой",
			args:       []string{"-p", ""},
			wantStderr: "gms: в -p не указан ни один код проекта, пример: -p project1,project3\n",
		},
		{
			name:       "-p только с запятыми и пробелами",
			args:       []string{"-p", " , "},
			wantStderr: "gms: в -p не указан ни один код проекта, пример: -p project1,project3\n",
		},
		{
			name:       "неизвестный код",
			args:       []string{"-p", "project1,foo"},
			wantStderr: "gms: неизвестный код проекта: foo\nизвестные коды: project2, project1\n",
		},
		{
			name:       "несколько неизвестных кодов",
			args:       []string{"-p", "foo,project1", "-p", "bar"},
			wantStderr: "gms: неизвестные коды проектов: foo, bar\nизвестные коды: project2, project1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runWithEnv(t, configEnv(t, twoProjectsConfig), forbiddenGlab(t), tt.args)
			assertResult(t, code, stdout, stderr, 2, "", tt.wantStderr)
		})
	}
}
