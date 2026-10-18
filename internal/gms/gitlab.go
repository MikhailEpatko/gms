package gms

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

// batchSize — сколько пар запрашивается одним запросом GraphQL. Сложность запроса растёт с числом пар
// (около 12 на пару в запросе деплоев и около 15 в запросе версий), а GitLab отклоняет запросы сложнее 250.
// Пачки нужны потому, что каждый запуск glab дорогой: антивирус проверяет его бинарник при каждом запуске.
const batchSize = 8

// errNotLoggedIn — GitLab ответил на запрос glab анонимно.
var errNotLoggedIn = errors.New("GitLab не узнал пользователя glab")

// Deployment — текущий деплой окружения.
type Deployment struct {
	Ref        string
	SHA        string
	FinishedAt time.Time // дата деплоя
	ShortID    string
	Title      string
}

// gitlab выполняет запросы к GitLab через glab api graphql (ADR 0001).
type gitlab struct {
	glab func(args ...string) ([]byte, error)
}

// unusableError — glab непригоден для хоста: не установлен или не вошёл в него.
type unusableError struct {
	host string
	err  error
}

func (e *unusableError) Error() string { return e.host + ": " + e.err.Error() }

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

// pairs запрашивает текущий деплой каждого окружения envs и версию приложения на его коммите.
// Пары одного хоста запрашиваются пачками, пачки — одновременно. Ошибки запросов попадают в результаты пар,
// а ошибка *unusableError возвращается, если glab непригоден хотя бы для одного хоста: тогда результатов нет.
func (g gitlab) pairs(envs []Environment) ([]pairResult, error) {
	batches := batches(envs)
	results := make([]pairResult, len(envs))
	errs := make([]error, len(batches))
	var wg sync.WaitGroup
	for i, idx := range batches {
		wg.Go(func() {
			batch := make([]Environment, len(idx))
			for j, k := range idx {
				batch[j] = envs[k]
			}
			rs, err := g.batch(batch)
			if err != nil {
				errs[i] = &unusableError{host: batch[0].Host, err: err}
				return
			}
			for j, k := range idx {
				results[k] = rs[j]
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// batches делит индексы envs на пачки до batchSize пар одного хоста. Хосты идут в порядке первого упоминания.
func batches(envs []Environment) [][]int {
	var hosts []string
	byHost := map[string][]int{}
	for i, env := range envs {
		if _, ok := byHost[env.Host]; !ok {
			hosts = append(hosts, env.Host)
		}
		byHost[env.Host] = append(byHost[env.Host], i)
	}
	var bs [][]int
	for _, h := range hosts {
		bs = slices.AppendSeq(bs, slices.Chunk(byHost[h], batchSize))
	}
	return bs
}

// batch запрашивает текущие деплои окружений envs одного хоста одним запросом, затем версии на их коммитах вторым.
// Возвращает ошибку, только если glab не установлен или не вошёл в хост.
func (g gitlab) batch(envs []Environment) ([]pairResult, error) {
	results := make([]pairResult, len(envs))
	deployments, err := g.currentDeployments(envs)
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, errNotLoggedIn) {
		return nil, err
	}
	var deployed []int // пары с текущим деплоем, им нужна версия
	for i := range results {
		switch {
		case err != nil:
			results[i].err = err
		case deployments[i].err != nil:
			results[i].err = deployments[i].err
		case deployments[i].deployment != nil:
			results[i].deployment = deployments[i].deployment
			deployed = append(deployed, i)
		}
	}
	if len(deployed) == 0 {
		return results, nil
	}
	versionEnvs := make([]Environment, len(deployed))
	shas := make([]string, len(deployed))
	for j, i := range deployed {
		versionEnvs[j], shas[j] = envs[i], results[i].deployment.SHA
	}
	versions, err := g.appVersions(versionEnvs, shas)
	for j, i := range deployed {
		if err != nil {
			results[i].versionErr = err
			continue
		}
		results[i].version, results[i].versionErr = versions[j].version, versions[j].err
	}
	return results, nil
}

// graphql выполняет запрос query к хосту host и разбирает поле data ответа: алиас → JSON поля.
func (g gitlab) graphql(host, query string) (map[string]json.RawMessage, error) {
	out, err := g.glab("api", "--hostname", host, "graphql", "-f", "query="+query)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("разбор ответа GitLab: %w", err)
	}
	if resp.Data == nil {
		return nil, errors.New("в ответе GitLab нет data")
	}
	return resp.Data, nil
}

// graphqlString записывает s строковым литералом GraphQL: его экранирование совпадает с JSON.
func graphqlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// projectAlias — алиас проекта пары i в запросе. У каждой пары свой проект, даже если путь повторяется.
func projectAlias(i int) string { return fmt.Sprintf("p%d", i) }

// deploymentResult — текущий деплой пары или ошибка разбора её ответа.
type deploymentResult struct {
	deployment *Deployment // nil — у окружения нет деплоев
	err        error
}

// currentDeployments возвращает текущий деплой каждого окружения envs: последний успешный деплой из любой
// ветки (ADR 0002). Тем же запросом проверяется, что glab вошёл в хост: иначе возвращается errNotLoggedIn.
func (g gitlab) currentDeployments(envs []Environment) ([]deploymentResult, error) {
	var q strings.Builder
	q.WriteString("query { currentUser { username }")
	for i, env := range envs {
		fmt.Fprintf(&q, " %s: project(fullPath: %s) { e: environment(name: %s) {"+
			" lastDeployment(status: SUCCESS) { ref commit { sha shortId title } job { finishedAt } } } }",
			projectAlias(i), graphqlString(env.ProjectPath), graphqlString(strings.ToLower(env.Name)))
	}
	q.WriteString(" }")
	data, err := g.graphql(envs[0].Host, q.String())
	if err != nil {
		return nil, err
	}
	// Анонимен только явный null: ответ без data — это ошибка ответа, а не отсутствие входа.
	if string(data["currentUser"]) == "null" {
		return nil, errNotLoggedIn
	}
	results := make([]deploymentResult, len(envs))
	for i, env := range envs {
		var project *struct {
			Environment *struct {
				LastDeployment *struct {
					Ref    string `json:"ref"`
					Commit *struct {
						SHA     string `json:"sha"`
						ShortID string `json:"shortId"`
						Title   string `json:"title"`
					} `json:"commit"`
					Job *struct {
						FinishedAt time.Time `json:"finishedAt"`
					} `json:"job"`
				} `json:"lastDeployment"`
			} `json:"e"`
		}
		if err := json.Unmarshal(orNull(data[projectAlias(i)]), &project); err != nil {
			results[i].err = fmt.Errorf("разбор ответа GitLab: %w", err)
			continue
		}
		if project == nil {
			results[i].err = notFound(env)
			continue
		}
		if project.Environment == nil || project.Environment.LastDeployment == nil {
			continue
		}
		d := project.Environment.LastDeployment
		// Без SHA не запросить версию, без даты деплоя вывод бессмыслен. Даты нет и тогда, когда нет job.
		switch {
		case d.Commit == nil || d.Commit.SHA == "":
			results[i].err = errors.New("в ответе GitLab нет commit.sha")
		case d.Job == nil || d.Job.FinishedAt.IsZero():
			results[i].err = errors.New("в ответе GitLab нет job.finishedAt")
		default:
			results[i].deployment = &Deployment{
				Ref:        d.Ref,
				SHA:        d.Commit.SHA,
				FinishedAt: d.Job.FinishedAt,
				ShortID:    d.Commit.ShortID,
				Title:      d.Commit.Title,
			}
		}
	}
	return results, nil
}

// versionResult — версия приложения пары или её ошибка.
type versionResult struct {
	version string
	err     error
}

// appVersions читает build_version из application.version окружения envs[i] на коммите shas[i].
func (g gitlab) appVersions(envs []Environment, shas []string) ([]versionResult, error) {
	var q strings.Builder
	q.WriteString("query {")
	for i, env := range envs {
		fmt.Fprintf(&q, " %s: project(fullPath: %s) { v: repository {"+
			" blobs(ref: %s, paths: [\"application.version\"]) { nodes { rawBlob } } } }",
			projectAlias(i), graphqlString(env.ProjectPath), graphqlString(shas[i]))
	}
	q.WriteString(" }")
	data, err := g.graphql(envs[0].Host, q.String())
	if err != nil {
		return nil, err
	}
	versions := make([]versionResult, len(envs))
	for i, env := range envs {
		var project *struct {
			Repository *struct {
				Blobs struct {
					Nodes []struct {
						RawBlob string `json:"rawBlob"`
					} `json:"nodes"`
				} `json:"blobs"`
			} `json:"v"`
		}
		switch err := json.Unmarshal(orNull(data[projectAlias(i)]), &project); {
		case err != nil:
			versions[i].err = fmt.Errorf("разбор ответа GitLab: %w", err)
		case project == nil:
			versions[i].err = notFound(env)
		case project.Repository == nil || len(project.Repository.Blobs.Nodes) == 0:
			versions[i].err = errors.New("на коммите деплоя нет application.version")
		default:
			versions[i].version, versions[i].err = buildVersion(project.Repository.Blobs.Nodes[0].RawBlob)
		}
	}
	return versions, nil
}

// buildVersion возвращает значение build_version из содержимого application.version.
func buildVersion(file string) (string, error) {
	for _, line := range strings.Split(file, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "build_version="); ok {
			if v == "" {
				return "", errors.New("в application.version пустой build_version")
			}
			return v, nil
		}
	}
	return "", errors.New("в application.version нет build_version")
}

// notFound — ошибка пары, проект которой GitLab не вернул: GraphQL отвечает null и на чужой проект.
func notFound(env Environment) error {
	return fmt.Errorf("проект %s не найден в GitLab или к нему нет доступа", env.ProjectPath)
}

// orNull заменяет отсутствующее поле ответа на null.
func orNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}
