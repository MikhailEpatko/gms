package gms

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Project — проект из конфига с окружениями в порядке файла.
type Project struct {
	Code         string
	Environments []Environment
}

// Environment — окружение проекта, разобранное из URL ветки окружения.
type Environment struct {
	Name        string // ключ окружения как в конфиге, например DEV
	Host        string // хост GitLab
	ProjectPath string // путь проекта в GitLab, например group/project1
	Branch      string // ветка окружения
}

// configPath возвращает путь к конфигу: из GMS_CONFIG, а без неё — в домашнем каталоге.
func configPath(getenv func(string) string) (string, error) {
	if p := getenv("GMS_CONFIG"); p != "" {
		return p, nil
	}
	home := getenv("HOME")
	if home == "" {
		// Иначе путь стал бы относительным и зависел от текущего каталога.
		return "", errors.New("не задан HOME, укажите путь к конфигу в GMS_CONFIG")
	}
	return filepath.Join(home, ".config", "gms", "config.json"), nil
}

func loadConfig(path string) ([]Project, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	projects, err := parseConfig(data)
	if err != nil {
		// Окружение синтаксической ошибки JSON известно не всегда, поэтому указываем строку.
		where := path
		if se, ok := errors.AsType[*json.SyntaxError](err); ok {
			where += ":" + strconv.Itoa(1+bytes.Count(data[:se.Offset], []byte("\n")))
		}
		return nil, fmt.Errorf("некорректный конфиг %s: %w", where, err)
	}
	return projects, nil
}

// parseConfig читает JSON потоком токенов, чтобы сохранить порядок ключей на обоих уровнях.
// Ошибка указывает проект и окружение, в которых она найдена.
func parseConfig(data []byte) ([]Project, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("файл пуст")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	var projects []Project
	err := decodeObject(dec, "", func(code string) error {
		p := Project{Code: code}
		err := decodeObject(dec, code, func(name string) error {
			var raw string
			if err := dec.Decode(&raw); err != nil {
				return fmt.Errorf("%s.%s: %w", code, name, err)
			}
			env, err := parseBranchURL(raw)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", code, name, err)
			}
			env.Name = name
			p.Environments = append(p.Environments, env)
			return nil
		})
		if err != nil {
			return err
		}
		if len(p.Environments) == 0 {
			return fmt.Errorf("%s: нет ни одного окружения", code)
		}
		projects = append(projects, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("лишние данные после объекта конфига")
	}
	if len(projects) == 0 {
		return nil, errors.New("нет ни одного проекта")
	}
	return projects, nil
}

// decodeObject проходит по ключам JSON-объекта в порядке файла и вызывает value для каждого ключа,
// который должен прочитать значение из dec. Ошибки разбора самого объекта предваряются errPrefix, если он
// не пуст; ошибки value возвращаются как есть.
func decodeObject(dec *json.Decoder, errPrefix string, value func(key string) error) error {
	wrap := func(err error) error {
		if err == nil || errPrefix == "" {
			return err
		}
		return fmt.Errorf("%s: %w", errPrefix, err)
	}
	if err := expectDelim(dec, '{'); err != nil {
		return wrap(err)
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return wrap(err)
		}
		if err := value(tok.(string)); err != nil {
			return err
		}
	}
	return wrap(expectDelim(dec, '}'))
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != want {
		return fmt.Errorf("ожидался %q, получено %v", want, tok)
	}
	return nil
}

// parseBranchURL разбирает URL вида https://<host>/<group>/<repo>/-/tree/<branch>.
func parseBranchURL(raw string) (Environment, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Environment{}, err
	}
	projectPath, branch, ok := strings.Cut(strings.Trim(u.Path, "/"), "/-/tree/")
	if u.Scheme != "https" || u.Host == "" || !ok || !strings.Contains(projectPath, "/") || branch == "" {
		return Environment{}, fmt.Errorf("URL %q не вида https://<host>/<group>/<repo>/-/tree/<branch>", raw)
	}
	return Environment{Host: u.Host, ProjectPath: projectPath, Branch: branch}, nil
}
