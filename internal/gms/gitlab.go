package gms

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Deployment — текущий деплой окружения.
type Deployment struct {
	Ref        string
	SHA        string
	FinishedAt time.Time // дата деплоя
	ShortID    string
	Title      string
}

// gitlab выполняет запросы к GitLab через glab api (ADR 0001).
type gitlab struct {
	glab func(args ...string) ([]byte, error)
}

// checkAuth проверяет до запросов к API, что glab установлен и залогинен в host.
// Если glab не найден, ошибка оборачивает exec.ErrNotFound.
func (g gitlab) checkAuth(host string) error {
	_, err := g.glab("auth", "status", "--hostname", host)
	return err
}

func (g gitlab) api(env Environment, path string) ([]byte, error) {
	return g.glab("api", "--hostname", env.Host, "projects/"+url.PathEscape(env.ProjectPath)+path)
}

// currentDeployment возвращает текущий деплой окружения: самый поздний успешный деплой из любой ветки (ADR 0002).
// Если у окружения нет ни одного деплоя, возвращает nil.
func (g gitlab) currentDeployment(env Environment) (*Deployment, error) {
	out, err := g.api(env, "/deployments?environment="+url.QueryEscape(strings.ToLower(env.Name))+
		"&status=success&order_by=finished_at&sort=desc&per_page=1")
	if err != nil {
		return nil, err
	}
	var resp []struct {
		Ref        string `json:"ref"`
		SHA        string `json:"sha"`
		Deployable struct {
			FinishedAt time.Time `json:"finished_at"`
			Commit     struct {
				ShortID string `json:"short_id"`
				Title   string `json:"title"`
			} `json:"commit"`
		} `json:"deployable"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("разбор ответа Deployments API: %w", err)
	}
	if len(resp) == 0 {
		return nil, nil
	}
	d := resp[0]
	// Без SHA не запросить версию, без даты деплоя вывод бессмыслен. Даты нет и тогда, когда нет deployable.
	switch {
	case d.SHA == "":
		return nil, errors.New("в ответе Deployments API нет sha")
	case d.Deployable.FinishedAt.IsZero():
		return nil, errors.New("в ответе Deployments API нет deployable.finished_at")
	}
	return &Deployment{
		Ref:        d.Ref,
		SHA:        d.SHA,
		FinishedAt: d.Deployable.FinishedAt,
		ShortID:    d.Deployable.Commit.ShortID,
		Title:      d.Deployable.Commit.Title,
	}, nil
}

// appVersion читает build_version из application.version на коммите sha.
func (g gitlab) appVersion(env Environment, sha string) (string, error) {
	out, err := g.api(env, "/repository/files/application.version/raw?ref="+url.QueryEscape(sha))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "build_version="); ok {
			if v == "" {
				return "", errors.New("в application.version пустой build_version")
			}
			return v, nil
		}
	}
	return "", errors.New("в application.version нет build_version")
}

// pairResult — то, что gitlab узнал о паре проект+окружение.
type pairResult struct {
	deployment *Deployment // текущий деплой, nil — у окружения нет деплоев
	err        error       // ошибка запроса текущего деплоя
	version    string
	versionErr error
}

// failed сообщает, что пара завершилась ошибкой: текущего деплоя или версии.
func (r pairResult) failed() bool {
	return r.err != nil || r.versionErr != nil
}

// pair запрашивает текущий деплой окружения env и версию приложения на его коммите.
func (g gitlab) pair(env Environment) pairResult {
	d, err := g.currentDeployment(env)
	if err != nil || d == nil {
		return pairResult{err: err}
	}
	version, versionErr := g.appVersion(env, d.SHA)
	return pairResult{deployment: d, version: version, versionErr: versionErr}
}
