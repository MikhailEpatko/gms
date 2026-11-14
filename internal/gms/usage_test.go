package gms

import (
	"cmp"
	"strconv"
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
			want: "{\"config\":\"/home/dev/.config/gms/config.json\"}\n",
		},
		{
			name: "без HOME в домашнем каталоге ОС",
			env:  map[string]string{"USERPROFILE": "/users/dev"},
			args: []string{"-c"},
			want: "{\"config\":\"/users/dev/.config/gms/config.json\"}\n",
		},
		{
			name: "HOME важнее домашнего каталога ОС",
			env:  map[string]string{"HOME": "/home/dev", "USERPROFILE": "/users/dev"},
			args: []string{"-c"},
			want: "{\"config\":\"/home/dev/.config/gms/config.json\"}\n",
		},
		{
			name: "из GMS_CONFIG",
			env:  map[string]string{"HOME": "/home/dev", "GMS_CONFIG": "/etc/gms.json"},
			args: []string{"-c"},
			want: "{\"config\":\"/etc/gms.json\"}\n",
		},
		{
			name: "с двумя дефисами",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"--c"},
			want: "{\"config\":\"/home/dev/.config/gms/config.json\"}\n",
		},
		{
			name: "со значением true",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"-c=true"},
			want: "{\"config\":\"/home/dev/.config/gms/config.json\"}\n",
		},
		{
			name: "другие флаги и аргументы игнорируются",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"-p", "foo", "-c", "-p", ",", "project1"},
			want: "{\"config\":\"/home/dev/.config/gms/config.json\"}\n",
		},
		{
			name: "-e игнорируется",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"-e", "DEV", "-c", "-e", ","},
			want: "{\"config\":\"/home/dev/.config/gms/config.json\"}\n",
		},
		{
			name: "после позиционного аргумента и неизвестного флага",
			env:  map[string]string{"HOME": "/home/dev"},
			args: []string{"-x", "-p", "project1", "project3", "-c"},
			want: "{\"config\":\"/home/dev/.config/gms/config.json\"}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runWithEnv(t, tt.env, forbiddenGlab(t), tt.args)
			assertResult(t, code, stdout, stderr, 0, tt.want, "")
		})
	}
}

const wantUsage = `использование: gms [-p код,код] [-e имя,имя] [-c]

Показывает текущий деплой каждого окружения проектов из конфига:
дату деплоя, версию приложения и коммит деплоя.
Вывод — JSON: отчёт в stdout, ошибки и подсказки в stderr.

  -p код,код  только перечисленные проекты, в порядке аргументов
  -e имя,имя  только перечисленные окружения, в порядке аргументов, регистр не важен
  -c          напечатать путь к конфигу и выйти
  -h          показать эту справку

Конфиг: ~/.config/gms/config.json или путь из переменной GMS_CONFIG.
`

func usageMsg() string {
	return "{\"usage\":" + strconv.Quote(wantUsage) + "}\n"
}

func usageErrorMsg(err string) string {
	return "{\"error\":" + strconv.Quote(err) + ",\"usage\":" + strconv.Quote(wantUsage) + "}\n"
}

func TestRunShowsHelp(t *testing.T) {
	code, stdout, stderr := runWithEnv(t, configEnv(t, twoProjectsConfig), forbiddenGlab(t), []string{"-h"})
	assertResult(t, code, stdout, stderr, 0, usageMsg(), "")
}

const envNamesConfig = `{
  "project2": {
    "dev": "https://gitlab.example.com/group/project2/-/tree/dev",
    "PROD": "https://gitlab.example.com/group/project2/-/tree/main"
  },
  "project3": {
    "DEV": "https://gitlab.example.com/group/project3/-/tree/dev",
    "PROD": "https://gitlab.example.com/group/project3/-/tree/main"
  },
  "project1": {
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project1/-/tree/demo",
    "DMZ": "https://gitlab.example.com/group/project1/-/tree/dmz"
  }
}`

func TestRunRejectsUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		args       []string
		wantStderr string
	}{
		{
			name:       "неизвестный флаг",
			args:       []string{"-x"},
			wantStderr: usageErrorMsg("flag provided but not defined: -x"),
		},
		{
			name:       "-c с некорректным значением",
			args:       []string{"-c=yes"},
			wantStderr: usageErrorMsg("invalid boolean value \"yes\" for -c: parse error"),
		},
		{
			name:       "коды через пробел после -p",
			args:       []string{"-p", "project1", "project3"},
			wantStderr: "{\"error\":\"лишние аргументы: project3\",\"hint\":\"коды проектов перечисляются через запятую: gms -p project1,project3\"}\n",
		},
		{
			name:       "код без -p",
			args:       []string{"project1"},
			wantStderr: "{\"error\":\"лишние аргументы: project1\",\"hint\":\"коды проектов перечисляются через запятую: gms -p project1\"}\n",
		},
		{
			name:       "код без -p перед -p",
			args:       []string{"project1", "-p", "project3"},
			wantStderr: "{\"error\":\"лишние аргументы: project1 -p project3\",\"hint\":\"коды проектов перечисляются через запятую: gms -p project1,project3\"}\n",
		},
		{
			name:       "имена окружений через пробел после -e",
			args:       []string{"-e", "DEV", "DEMO"},
			wantStderr: "{\"error\":\"лишние аргументы: DEMO\",\"hint\":\"окружения перечисляются через запятую: gms -e DEV,DEMO\"}\n",
		},
		{
			name:       "коды и имена через пробел",
			args:       []string{"-p", "project1", "project3", "-e", "PROD", "DEV"},
			wantStderr: "{\"error\":\"лишние аргументы: project3 -e PROD DEV\",\"hint\":\"коды проектов и окружения перечисляются через запятую: gms -p project1,project3 -e PROD,DEV\"}\n",
		},
		{
			name:       "в подсказке -p перед -e",
			args:       []string{"-e", "DEV", "-p", "project1", "project3"},
			wantStderr: "{\"error\":\"лишние аргументы: project3\",\"hint\":\"коды проектов и окружения перечисляются через запятую: gms -p project1,project3 -e DEV\"}\n",
		},
		{
			name:       "записи -e= и --p",
			args:       []string{"-e=DEV", "DEMO", "--p", "project1", "project3"},
			wantStderr: "{\"error\":\"лишние аргументы: DEMO --p project1 project3\",\"hint\":\"коды проектов и окружения перечисляются через запятую: gms -p project1,project3 -e DEV,DEMO\"}\n",
		},
		{
			name:       "записи --e и --p=",
			args:       []string{"--e", "DEV", "DEMO", "--p=project1", "project3"},
			wantStderr: "{\"error\":\"лишние аргументы: DEMO --p=project1 project3\",\"hint\":\"коды проектов и окружения перечисляются через запятую: gms -p project1,project3 -e DEV,DEMO\"}\n",
		},
		{
			name:       "другие флаги не попадают в подсказку и не меняют флаг слов",
			args:       []string{"-e", "DEV", "-c=false", "DEMO", "-x", "PROD"},
			wantStderr: "{\"error\":\"лишние аргументы: DEMO -x PROD\",\"hint\":\"окружения перечисляются через запятую: gms -e DEV,DEMO,PROD\"}\n",
		},
		{
			name:       "повторы кодов в подсказке схлопываются",
			args:       []string{"-p", "project1", "project1,project3,"},
			wantStderr: "{\"error\":\"лишние аргументы: project1,project3,\",\"hint\":\"коды проектов перечисляются через запятую: gms -p project1,project3\"}\n",
		},
		{
			name:       "повторы имён в подсказке схлопываются без учёта регистра",
			args:       []string{"-e", "dev, ", "DEV,DEMO"},
			wantStderr: "{\"error\":\"лишние аргументы: DEV,DEMO\",\"hint\":\"окружения перечисляются через запятую: gms -e dev,DEMO\"}\n",
		},
		{
			name:       "лишний аргумент без значений",
			args:       []string{","},
			wantStderr: "{\"error\":\"лишние аргументы: ,\",\"hint\":\"коды проектов перечисляются через запятую: gms\"}\n",
		},
		{
			name:       "флаг после -p в подсказке не значение",
			args:       []string{"-p", "-e", "DEV"},
			wantStderr: "{\"error\":\"лишние аргументы: DEV\",\"hint\":\"окружения перечисляются через запятую: gms -e DEV\"}\n",
		},
		{
			name:       "-p с пустой строкой",
			args:       []string{"-p", ""},
			wantStderr: "{\"error\":\"в -p не указан ни один код проекта\",\"hint\":\"пример: -p project1,project3\"}\n",
		},
		{
			name:       "-p только с запятыми и пробелами",
			args:       []string{"-p", " , "},
			wantStderr: "{\"error\":\"в -p не указан ни один код проекта\",\"hint\":\"пример: -p project1,project3\"}\n",
		},
		{
			name:       "-e с пустой строкой",
			args:       []string{"-e", ""},
			wantStderr: "{\"error\":\"в -e не указано ни одно окружение\",\"hint\":\"пример: -e DEV,DEMO\"}\n",
		},
		{
			name:       "-e только с запятыми и пробелами",
			args:       []string{"-e", " , ", "-p", "project1"},
			wantStderr: "{\"error\":\"в -e не указано ни одно окружение\",\"hint\":\"пример: -e DEV,DEMO\"}\n",
		},
		{
			name:       "неизвестный код",
			args:       []string{"-p", "project1,foo"},
			wantStderr: "{\"error\":\"неизвестный код проекта: foo\",\"known\":[\"project2\",\"project1\"]}\n",
		},
		{
			name:       "несколько неизвестных кодов",
			args:       []string{"-p", "foo,project1", "-p", "bar"},
			wantStderr: "{\"error\":\"неизвестные коды проектов: foo, bar\",\"known\":[\"project2\",\"project1\"]}\n",
		},
		{
			name:       "неизвестное окружение",
			config:     envNamesConfig,
			args:       []string{"-e", "PRDO"},
			wantStderr: "{\"error\":\"неизвестное окружение: PRDO\",\"known\":[\"dev\",\"PROD\",\"DEMO\",\"DMZ\"]}\n",
		},
		{
			name:       "несколько неизвестных окружений",
			config:     envNamesConfig,
			args:       []string{"-e", "PRDO,dev", "-e", "stage"},
			wantStderr: "{\"error\":\"неизвестные окружения: PRDO, stage\",\"known\":[\"dev\",\"PROD\",\"DEMO\",\"DMZ\"]}\n",
		},
		{
			name:   "неизвестные коды и окружения вместе",
			config: envNamesConfig,
			args:   []string{"-p", "foo", "-e", "PRDO"},
			wantStderr: "{\"error\":\"неизвестный код проекта: foo\",\"known\":[\"project2\",\"project3\",\"project1\"]}\n" +
				"{\"error\":\"неизвестное окружение: PRDO\",\"known\":[\"dev\",\"PROD\",\"DEMO\",\"DMZ\"]}\n",
		},
		{
			name:       "известные окружения по всем проектам конфига, а не только выбранным",
			config:     envNamesConfig,
			args:       []string{"-p", "project2", "-e", "DEMO,PRDO"},
			wantStderr: "{\"error\":\"неизвестное окружение: PRDO\",\"known\":[\"dev\",\"PROD\",\"DEMO\",\"DMZ\"]}\n",
		},
		{
			name:       "у проекта нет окружения",
			config:     envNamesConfig,
			args:       []string{"-e", "DEMO", "-p", "project2"},
			wantStderr: "{\"error\":\"у проекта project2 нет окружения DEMO\"}\n",
		},
		{
			name:       "у проекта нет окружений",
			config:     envNamesConfig,
			args:       []string{"-e", "DEMO,DMZ", "-p", "project2"},
			wantStderr: "{\"error\":\"у проекта project2 нет окружений DEMO, DMZ\"}\n",
		},
		{
			name:       "у проектов нет окружения",
			config:     envNamesConfig,
			args:       []string{"-e", "DEMO", "-p", "project2,project3"},
			wantStderr: "{\"error\":\"у проектов project2, project3 нет окружения DEMO\"}\n",
		},
		{
			name:       "у проектов нет окружений, имена как в аргументах",
			config:     envNamesConfig,
			args:       []string{"-e", "DEMO,dmz", "-p", "project2,project3"},
			wantStderr: "{\"error\":\"у проектов project2, project3 нет окружений DEMO, dmz\"}\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runWithEnv(t, configEnv(t, cmp.Or(tt.config, twoProjectsConfig)), forbiddenGlab(t), tt.args)
			assertResult(t, code, stdout, stderr, 2, "", tt.wantStderr)
		})
	}
}
