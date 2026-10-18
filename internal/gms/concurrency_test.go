package gms

import (
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// allPairsDeployments — запросы деплоев всех пар allPairsConfig в порядке вывода.
var allPairsDeployments = []string{
	deploymentsCall("group%2Fproject2", "dev"),
	deploymentsCall("group%2Fproject2", "demo"),
	deploymentsCall("group%2Fproject2", "prod"),
	deploymentsCall("group%2Fproject1", "dev"),
	deploymentsCall("group%2Fproject1", "demo"),
	deploymentsCall("group%2Fproject1", "prod"),
}

// reversedGlab отвечает на запросы calls в обратном порядке: ответ на calls[i] glab возвращает
// только после ответа на calls[i+1]. Остальные вызовы передаются glab без ожидания.
// Если calls[i+1] не запрошен за секунду, то есть запросы идут не параллельно, calls[i] завершается ошибкой.
func reversedGlab(glab func(args ...string) ([]byte, error), calls []string) func(args ...string) ([]byte, error) {
	answered := make(map[string]chan struct{}, len(calls))
	for _, c := range calls {
		answered[c] = make(chan struct{})
	}
	return func(args ...string) ([]byte, error) {
		key := strings.Join(args, " ")
		i := slices.Index(calls, key)
		if i < 0 {
			return glab(args...)
		}
		defer close(answered[key])
		if i+1 < len(calls) {
			select {
			case <-answered[calls[i+1]]:
			case <-time.After(time.Second):
				return nil, errors.New("не дождался ответа на следующий запрос")
			}
		}
		return glab(args...)
	}
}

// barrierGlab держит каждый запрос деплоя, пока не придут все n запросов деплоя, и так же держит запросы версии.
// Остальные вызовы передаются glab без ожидания. Если за секунду n запросов одного вида не собрались,
// то есть запросы этого вида идут не одновременно, glab возвращает ошибку.
func barrierGlab(glab func(args ...string) ([]byte, error), n int) func(args ...string) ([]byte, error) {
	var mu sync.Mutex
	arrived := map[string]int{}
	all := map[string]chan struct{}{"deployments": make(chan struct{}), "version": make(chan struct{})}
	return func(args ...string) ([]byte, error) {
		if args[0] != "api" {
			return glab(args...)
		}
		kind := "version"
		if strings.Contains(args[len(args)-1], "/deployments?") {
			kind = "deployments"
		}
		mu.Lock()
		if arrived[kind]++; arrived[kind] == n {
			close(all[kind])
		}
		mu.Unlock()
		select {
		case <-all[kind]:
		case <-time.After(time.Second):
			return nil, errors.New("не дождался остальных запросов того же вида")
		}
		return glab(args...)
	}
}

func TestRunKeepsOutputOrderWhenAnswersComeReversed(t *testing.T) {
	tests := []struct {
		name     string
		errs     glabErrors
		wantCode int
		want     string
	}{
		{
			name:     "все пары успешны",
			wantCode: 0,
			want:     project2All + projectSep + project1All,
		},
		{
			name: "ошибки пары и версии",
			errs: glabErrors{
				deploymentsCall("group%2Fproject2", "demo"):                                 errors.New("glab: 500 Internal Server Error (HTTP 500)"),
				versionCall("group%2Fproject1", "ad87e7d16ba3e32d5527166a81a47db41f3a56ba"): errors.New("glab: 404 File Not Found (HTTP 404)"),
			},
			wantCode: 1,
			want: "project2\n" + project2DevBlock + envSep + `DEMO:
ошибка: glab: 500 Internal Server Error (HTTP 500)
` + envSep + project2ProdBlock + projectSep + "project1\n" + project1DevBlock + envSep + `DEMO:
2026-10-16 19:02:37 MSK
версия: ошибка (glab: 404 File Not Found (HTTP 404))
ad87e7d1 Merge branch 'XXXXX-3602-main' into 'demo'
` + envSep + project1ProdBlock,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			glab := reversedGlab(fakeGlab(t, mergeResponses(project2Responses, project1Responses), tt.errs), allPairsDeployments)
			code, stdout, stderr := runWithEnv(t, configEnv(t, allPairsConfig), glab, nil)
			assertResult(t, code, stdout, stderr, tt.wantCode, tt.want, "")
		})
	}
}

// TestRunRequestsPairsConcurrently проверяет, что одновременно идут и все запросы деплоев, и все запросы версий.
func TestRunRequestsPairsConcurrently(t *testing.T) {
	glab := barrierGlab(fakeGlab(t, mergeResponses(project2Responses, project1Responses), nil), len(allPairsDeployments))
	code, stdout, stderr := runWithEnv(t, configEnv(t, allPairsConfig), glab, nil)
	assertResult(t, code, stdout, stderr, 0, project2All+projectSep+project1All, "")
}
