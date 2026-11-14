package gms

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

const wantExampleConfig = `{
  "project1": {
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project1/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project1/-/tree/main"
  },
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev",
    "PROD": "https://gitlab.example.com/group/project2/-/tree/main"
  }
}
`

func TestRunAsksToCreateMissingConfig(t *testing.T) {
	home := t.TempDir()
	custom := filepath.Join(t.TempDir(), "gms", "config.json")
	tests := []struct {
		name     string
		env      map[string]string
		wantPath string
	}{
		{
			name:     "по умолчанию в домашнем каталоге",
			env:      map[string]string{"HOME": home},
			wantPath: filepath.Join(home, ".config", "gms", "config.json"),
		},
		{
			name:     "из GMS_CONFIG",
			env:      map[string]string{"HOME": home, "GMS_CONFIG": custom},
			wantPath: custom,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runWithEnv(t, tt.env, forbiddenGlab(t), nil)
			wantStderr := "{\"error\":\"нет файла конфига\",\"path\":" + strconv.Quote(tt.wantPath) +
				",\"example\":" + strconv.Quote(wantExampleConfig) + "}\n"
			assertResult(t, code, stdout, stderr, 1, "", wantStderr)
			if _, err := os.Stat(filepath.Dir(tt.wantPath)); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("каталог конфига создан или недоступен: %v", err)
			}
		})
	}
}

func TestRunExampleConfigIsValid(t *testing.T) {
	code, stdout, stderr := runWithEnv(t, configEnv(t, wantExampleConfig), forbiddenGlab(t), []string{"-p", "foo"})
	assertResult(t, code, stdout, stderr, 2, "", "{\"error\":\"неизвестный код проекта: foo\",\"known\":[\"project1\",\"project2\"]}\n")
}

func TestRunNeedsHomeWithoutGmsConfig(t *testing.T) {
	for _, args := range [][]string{nil, {"-c"}} {
		t.Run(fmt.Sprint(args), func(t *testing.T) {
			code, stdout, stderr := runWithEnv(t, nil, forbiddenGlab(t), args)
			assertResult(t, code, stdout, stderr, 1, "", "{\"error\":\"не удалось определить домашний каталог, укажите путь к конфигу в GMS_CONFIG\"}\n")
		})
	}
}

func TestRunRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   string
		wantLine int
		wantErr  string
	}{
		{
			name:    "URL без /-/tree/",
			config:  `{"project2": {"DEV": "https://gitlab.example.com/group/project2"}}`,
			wantErr: `project2.DEV: URL "https://gitlab.example.com/group/project2" не вида https://<host>/<group>/<repo>/-/tree/<branch>`,
		},
		{
			name:    "URL без хоста",
			config:  `{"project2": {"DEV": "group/project2/-/tree/dev"}}`,
			wantErr: `project2.DEV: URL "group/project2/-/tree/dev" не вида https://<host>/<group>/<repo>/-/tree/<branch>`,
		},
		{
			name:    "URL не https",
			config:  `{"project2": {"DEV": "http://gitlab.example.com/group/project2/-/tree/dev"}}`,
			wantErr: `project2.DEV: URL "http://gitlab.example.com/group/project2/-/tree/dev" не вида https://<host>/<group>/<repo>/-/tree/<branch>`,
		},
		{
			name:    "URL без группы",
			config:  `{"project2": {"DEV": "https://gitlab.example.com/project2/-/tree/dev"}}`,
			wantErr: `project2.DEV: URL "https://gitlab.example.com/project2/-/tree/dev" не вида https://<host>/<group>/<repo>/-/tree/<branch>`,
		},
		{
			name:    "URL окружения не строка",
			config:  `{"project2": {"DEV": 5}}`,
			wantErr: "project2.DEV: json: cannot unmarshal number into Go value of type string",
		},
		{
			name: "пропущена запятая между окружениями",
			config: `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev"
    "DEMO": "https://gitlab.example.com/group/project2/-/tree/demo"
  }
}`,
			wantLine: 4,
			wantErr:  `project2: invalid character '"' after object key:value pair`,
		},
		{
			name:    "проект не объект",
			config:  `{"project2": "https://gitlab.example.com/group/project2/-/tree/dev"}`,
			wantErr: `project2: ожидался "{", получено https://gitlab.example.com/group/project2/-/tree/dev`,
		},
		{
			name:    "конфиг не объект",
			config:  `[]`,
			wantErr: `ожидался "{", получено [`,
		},
		{
			name:     "файл обрывается",
			config:   `{"project2": {"DEV": "https://gitlab.example.com/group/project2/-/tree/dev",`,
			wantLine: 1,
			wantErr:  "project2: unexpected end of JSON input",
		},
		{
			name:    "пустой файл",
			config:  ``,
			wantErr: "файл пуст",
		},
		{
			name:    "данные после объекта",
			config:  `{"project2": {"DEV": "https://gitlab.example.com/group/project2/-/tree/dev"}} {}`,
			wantErr: "лишние данные после объекта конфига",
		},
		{
			name:    "нет проектов",
			config:  `{}`,
			wantErr: "нет ни одного проекта",
		},
		{
			name:    "у проекта нет окружений",
			config:  `{"project2": {}}`,
			wantErr: "project2: нет ни одного окружения",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, tt.config)
			code, stdout, stderr := runWithEnv(t, map[string]string{"GMS_CONFIG": path}, forbiddenGlab(t), nil)
			where := path
			if tt.wantLine > 0 {
				where += fmt.Sprintf(":%d", tt.wantLine)
			}
			assertResult(t, code, stdout, stderr, 1, "", "{\"error\":"+strconv.Quote("некорректный конфиг "+where+": "+tt.wantErr)+"}\n")
		})
	}
}
