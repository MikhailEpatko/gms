package gms

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ninePairsConfig — девять пар, на одну больше, чем помещается в один запрос: project2 и project1 по три окружения
// и project3 с тремя окружениями проекта group/project2.
const ninePairsConfig = `{
  "project2": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project2/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project2/-/tree/main"
  },
  "project1": {
    "DEV": "https://gitlab.example.com/group/project1/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project1/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project1/-/tree/main"
  },
  "project3": {
    "DEV": "https://gitlab.example.com/group/project2/-/tree/dev",
    "DEMO": "https://gitlab.example.com/group/project2/-/tree/demo",
    "PROD": "https://gitlab.example.com/group/project2/-/tree/main"
  }
}`

// project3All — вывод project3 из ninePairsConfig: окружения group/project2 под кодом project3.
const project3All = "project3\n" + project2DevBlock + envSep + project2DemoBlock + envSep + project2ProdBlock

// queryKind возвращает вид запроса glab: деплои или версии.
func queryKind(args []string) string {
	if strings.Contains(strings.Join(args, " "), "environment(") {
		return "deployments"
	}
	return "versions"
}

// reversedGlab отвечает на n запросов каждого вида в порядке, обратном порядку их прихода: i-й пришедший запрос
// glab возвращает только после ответа на (i+1)-й. Если следующий запрос не пришёл за секунду, то есть запросы
// идут не параллельно, запрос завершается ошибкой.
func reversedGlab(glab func(args ...string) ([]byte, error), n int) func(args ...string) ([]byte, error) {
	var mu sync.Mutex
	arrived := map[string]int{}
	answered := map[string][]chan struct{}{}
	for _, kind := range []string{"deployments", "versions"} {
		for range n {
			answered[kind] = append(answered[kind], make(chan struct{}))
		}
	}
	return func(args ...string) ([]byte, error) {
		kind := queryKind(args)
		mu.Lock()
		i := arrived[kind]
		arrived[kind]++
		mu.Unlock()
		if i >= n {
			return glab(args...)
		}
		defer close(answered[kind][i])
		if i+1 < n {
			select {
			case <-answered[kind][i+1]:
			case <-time.After(time.Second):
				return nil, errors.New("не дождался ответа на следующий запрос")
			}
		}
		return glab(args...)
	}
}

// barrierGlab держит каждый запрос деплоев, пока не придут все n запросов деплоев, и так же держит запросы версий.
// Если за секунду n запросов одного вида не собрались, то есть запросы этого вида идут не одновременно,
// glab возвращает ошибку.
func barrierGlab(glab func(args ...string) ([]byte, error), n int) func(args ...string) ([]byte, error) {
	var mu sync.Mutex
	arrived := map[string]int{}
	all := map[string]chan struct{}{"deployments": make(chan struct{}), "versions": make(chan struct{})}
	return func(args ...string) ([]byte, error) {
		kind := queryKind(args)
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

// TestRunBatchesRequests проверяет, что glab запускается по разу на запрос деплоев и на запрос версий пачки
// до восьми пар: каждый запуск glab дорогой.
func TestRunBatchesRequests(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		want      string
		wantCalls int32
	}{
		{name: "одна пара", config: project2DevConfig, want: project2Dev, wantCalls: 2},
		{name: "шесть пар двух проектов", config: allPairsConfig, want: project2All + projectSep + project1All, wantCalls: 2},
		{name: "девять пар", config: ninePairsConfig, want: project2All + projectSep + project1All + projectSep + project3All,
			wantCalls: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls atomic.Int32
			glab := fakeGlab(t, testGitLab, glabFaults{})
			counting := func(args ...string) ([]byte, error) {
				calls.Add(1)
				return glab(args...)
			}
			code, stdout, stderr := runWithEnv(t, configEnv(t, tt.config), counting, nil)
			assertResult(t, code, stdout, stderr, 0, tt.want, "")
			if got := calls.Load(); got != tt.wantCalls {
				t.Errorf("glab запущен %d раз, want %d", got, tt.wantCalls)
			}
		})
	}
}

func TestRunKeepsOutputOrderWhenAnswersComeReversed(t *testing.T) {
	tests := []struct {
		name     string
		gl       fakeGitLab
		wantCode int
		want     string
	}{
		{
			name:     "все пары успешны",
			gl:       testGitLab,
			wantCode: 0,
			want:     project2All + projectSep + project1All + projectSep + project3All,
		},
		{
			name:     "ошибка пары",
			gl:       testGitLab.withDeployment("group/project1", "demo", "synthetic/last-deployment-no-job.json"),
			wantCode: 1,
			want: project2All + projectSep + "project1\n" + project1DevBlock + envSep + `DEMO:
ошибка: в ответе GitLab нет job.finishedAt
` + envSep + project1ProdBlock + projectSep + project3All,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			glab := reversedGlab(fakeGlab(t, tt.gl, glabFaults{}), 2)
			code, stdout, stderr := runWithEnv(t, configEnv(t, ninePairsConfig), glab, nil)
			assertResult(t, code, stdout, stderr, tt.wantCode, tt.want, "")
		})
	}
}

// TestRunRequestsBatchesConcurrently проверяет, что пачки запрашиваются одновременно: и запросы деплоев, и запросы версий.
func TestRunRequestsBatchesConcurrently(t *testing.T) {
	glab := barrierGlab(fakeGlab(t, testGitLab, glabFaults{}), 2)
	code, stdout, stderr := runWithEnv(t, configEnv(t, ninePairsConfig), glab, nil)
	assertResult(t, code, stdout, stderr, 0, project2All+projectSep+project1All+projectSep+project3All, "")
}
