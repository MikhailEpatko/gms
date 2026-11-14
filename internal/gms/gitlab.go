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

const batchSize = 8

var errNotLoggedIn = errors.New("GitLab не узнал пользователя glab")

type Deployment struct {
	SHA        string
	FinishedAt time.Time
	ShortID    string
	Title      string
}

type gitlab struct {
	glab func(args ...string) ([]byte, error)
}

type unusableError struct {
	host string
	err  error
}

func (e *unusableError) Error() string { return e.host + ": " + e.err.Error() }

type pairResult struct {
	deployment    *Deployment
	deploymentErr error
	version       string
	versionErr    error
}

func (r pairResult) failed() bool {
	return r.deploymentErr != nil || r.versionErr != nil
}

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

func (g gitlab) batch(envs []Environment) ([]pairResult, error) {
	results := make([]pairResult, len(envs))
	deployments, err := g.currentDeployments(envs)
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, errNotLoggedIn) {
		return nil, err
	}
	var deployedIdx []int
	for i := range results {
		switch {
		case err != nil:
			results[i].deploymentErr = err
		case deployments[i].err != nil:
			results[i].deploymentErr = deployments[i].err
		case deployments[i].deployment != nil:
			results[i].deployment = deployments[i].deployment
			deployedIdx = append(deployedIdx, i)
		}
	}
	if len(deployedIdx) == 0 {
		return results, nil
	}
	versionEnvs := make([]Environment, len(deployedIdx))
	shas := make([]string, len(deployedIdx))
	for j, i := range deployedIdx {
		versionEnvs[j], shas[j] = envs[i], results[i].deployment.SHA
	}
	versions, err := g.appVersions(versionEnvs, shas)
	for j, i := range deployedIdx {
		if err != nil {
			results[i].versionErr = err
			continue
		}
		results[i].version, results[i].versionErr = versions[j].version, versions[j].err
	}
	return results, nil
}

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

func graphqlString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func projectAlias(i int) string { return fmt.Sprintf("p%d", i) }

type deploymentResult struct {
	deployment *Deployment
	err        error
}

func (g gitlab) currentDeployments(envs []Environment) ([]deploymentResult, error) {
	var q strings.Builder
	q.WriteString("query { currentUser { username }")
	for i, env := range envs {
		_, _ = fmt.Fprintf(&q, " %s: project(fullPath: %s) { e: environment(name: %s) {"+
			" lastDeployment(status: SUCCESS) { commit { sha shortId title } job { finishedAt } } } }",
			projectAlias(i), graphqlString(env.ProjectPath), graphqlString(strings.ToLower(env.Name)))
	}
	q.WriteString(" }")
	data, err := g.graphql(envs[0].Host, q.String())
	if err != nil {
		return nil, err
	}
	if string(data["currentUser"]) == "null" {
		return nil, errNotLoggedIn
	}
	results := make([]deploymentResult, len(envs))
	for i, env := range envs {
		var project *struct {
			Environment *struct {
				LastDeployment *struct {
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
		switch {
		case d.Commit == nil || d.Commit.SHA == "":
			results[i].err = errors.New("в ответе GitLab нет commit.sha")
		case d.Job == nil || d.Job.FinishedAt.IsZero():
			results[i].err = errors.New("в ответе GitLab нет job.finishedAt")
		default:
			results[i].deployment = &Deployment{
				SHA:        d.Commit.SHA,
				FinishedAt: d.Job.FinishedAt,
				ShortID:    d.Commit.ShortID,
				Title:      d.Commit.Title,
			}
		}
	}
	return results, nil
}

type versionResult struct {
	version string
	err     error
}

func (g gitlab) appVersions(envs []Environment, shas []string) ([]versionResult, error) {
	var q strings.Builder
	q.WriteString("query {")
	for i, env := range envs {
		_, _ = fmt.Fprintf(&q, " %s: project(fullPath: %s) { v: repository {"+
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

func notFound(env Environment) error {
	return fmt.Errorf("проект %s не найден в GitLab или к нему нет доступа", env.ProjectPath)
}

func orNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}
