# AGENTS.md

Файл для ИИ-агентов, работающих с этим репозиторием. Проект документирован на русском — README, ADR, тесты и сообщения пользователю тоже на русском, поэтому файл ведётся на русском.

## Обзор проекта

`gms` (Give Me Stats) — CLI на Go, который для каждого проекта и каждого окружения показывает текущий статус деплоя: дату деплоя, версию приложения и коммит деплоя.

- Модуль `github.com/MikhailEpatko/gms`, Go 1.27 (см. `go.mod`), сторонних зависимостей **нет**.
- Все данные берутся из GitLab через подпроцесс `glab api --hostname <host> graphql` (ADR 0001–0003). Токенов в `gms` нет: авторизацию, хранение токена и работу с self-hosted хостами берёт на себя `glab`, который должен быть установлен и залогинен (`glab auth login --hostname <host>`, токен не ниже `read_api`).
- Конфиг пользователя: `~/.config/gms/config.json` (переопределяется env `GMS_CONFIG`) — JSON вида `{ "<код проекта>": { "<ОКРУЖЕНИЕ>": "<url ветки>" } }`, URL вида `https://gitlab.example.com/group/<repo>/-/tree/<branch>`.
- Коды выхода: `0` — успех, `1` — ошибка выполнения (конфиг, GitLab, провалившаяся пара), `2` — ошибка использования флагов.
- Полное описание поведения (флаги `-p`/`-e`/`-c`, формат вывода, обработка ошибок) — в `README.md`, разделы «Запуск»…«Ошибки». **README — источник правды по поведению утилиты.**

## Структура кода

```
main.go                  точка входа: подключает настоящий glab через exec.Command,
                         вызывает gms.Run(os.Args[1:], gms.Deps{...}); os.Exit по коду Run
internal/gms/            вся логика (пакет gms)
  run.go                 Run(args, deps): разбор флагов, выбор проектов/окружений, коды выхода
  config.go              путь к конфигу, разбор JSON с сохранением порядка ключей, парсинг URL ветки
  gitlab.go              запросы через glab api graphql, пачки до 8 пар на хост,
                         два запуска glab на пачку (деплои + версии), WaitGroup для параллельности
  format.go              весь JSON для пользователя: отчёт в stdout,
                         reporter (служебные сообщения в stderr), usage, пример конфига
  *_test.go              тесты по областям поведения; общие хелперы и фейк — в helpers_test.go
  testdata/              заготовки ответов GitLab (реальные + synthetic), см. testdata/README.md
docs/adr/                долговременные решения: 0001-glab, 0002-deployments-api,
                         0003-graphql-batches, 0004-secrets-checks, 0005-no-comments,
                         0006-json-output
scripts/check.sh         все проверки (см. ниже)
scripts/check-secrets.sh случаи правил gitleaks + сквозные проверки pre-commit hook
.githooks/pre-commit     gitleaks по индексу, затем check.sh на копии индекса
.github/workflows/       check.yml (на push в main и PR), release.yml (на тег v*)
ai/<N>/                  задачи: task.md → spec.md → ticket-<M>.md; N — порядок выполнения.
                         Где спека расходится с task.md, верна спека
```

Слои пакета `internal/gms` (подробности — `CODING_STANDARDS.md`):
- `gitlab.go` отвечает только за запросы через `glab` и разбор ответов; возвращает ошибки с причиной.
- JSON для пользователя собирается только в `format.go` (отчёт — stdout, reporter — stderr).
- `run.go` разбирает аргументы и решает, что выводить и с каким кодом выхода.

`Deps` в `Run` — единственный seam тестов: фейковый `Glab`, `Getenv`, `HomeDir`, буферы `Stdout`/`Stderr`, фиксированная зона `Location`.

## Команды

```bash
scripts/check.sh                          # все проверки, см. ниже — работа готова, только когда он прошёл
go build -o gms .                         # сборка бинарника
go run . -p project1,project2             # запуск без сборки
go test ./internal/gms -run TestName -v   # один тест
go test -race -count=10 -run <Test> ./internal/gms   # прогон гоночно-чувствительного теста
```

`scripts/check.sh` гоняет: `gofmt -l` → `nocomments` (запрет комментариев, ADR 0005) → `go vet ./...` → `golangci-lint run` → `go test -race -count=1 ./...` → `scripts/check-secrets.sh`. Требуются в `PATH`:
- `gitleaks` v8.30.1 (`go install github.com/zricethezav/gitleaks/v8@v8.30.1`),
- `nocomments` (`go install github.com/MikhailEpatko/nocomments@latest`),
- `golangci-lint` v2.15.0 (`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0`).

Pre-commit hook подключается в новом клоне: `git config core.hooksPath .githooks`. Сначала он проверяет индекс gitleaks (правила `.gitleaks.toml`), затем запускает `scripts/check.sh` на копии индекса во временном каталоге — проверяется ровно то, что попадёт в коммит. Коммитить только через hook, без `--no-verify`.

## Стиль кода

- **Комментариев в коде нет** (ADR 0005): ни в Go, ни в shell, ни в hook, ни в `.gitleaks.toml`. Проверяет `nocomments` в `check.sh`. Разрешены только директивы инструментов: `//go:…`, `//nolint…`, `// gitleaks:allow`, shebang. Знание живёт в именах, тестах, README, ADR, `CODING_STANDARDS.md`, `testdata/README.md`.
- Сообщения пользователю и имена тестовых случаев — на русском; идентификаторы Go — на английском. Термины — из `GLOSSARY.md` (проект, код проекта, окружение, имя окружения, ветка окружения, пара, деплой, текущий деплой, дата деплоя, коммит деплоя, версия приложения, ошибка пары, ошибка версии); слова из списков _Avoid_ не использовать.
- Линтер: `.golangci.yml` включает `errorlint` и `nolintlint`, `staticcheck` — all минус ST1000/ST1020–ST1022 (doc-комментарии запрещены). Замечание линтера чинится кодом; `//nolint:<имя>` — точечно на строку и только когда правило ошибается; правило целиком не отключается.
- Если diff меняет поведение, запросы к GitLab или термины — в том же diff обновлены README, ADR, GLOSSARY, при затрагивании процесса — `AGENTS.md`. ADR в репозитории «живые»: меняются на месте, а новые решения (трудно отменить, удивят без контекста, был реальный выбор) оформляются новым ADR.

## Тестирование

- **Один seam**: тесты вызывают `Run(args, Deps)` с фейковым glab, временным конфигом через `GMS_CONFIG`, буферами stdout/stderr и зоной `Europe/Moscow`. Проверяются только stdout, stderr и код выхода. Исключения — контрактные: число запусков glab и состав пар в запросах (ADR 0003, правило «в GitLab запрашиваются только выводимые пары»), их наблюдает обёртка над фейком (`TestRunBatchesRequests`, `TestRunRequestsOnlySelectedPairs`). Внутренние функции отдельно не тестируются.
- **Без сети**: ни один тест не обращается к настоящему GitLab.
- Таблицы кейсов — на стандартном `testing`, без сторонних библиотек. Ожидаемые значения — литералами, а не собраны теми же функциями/константами, что код; повторяющиеся блоки — константы в `helpers_test.go` (`project2Dev` и т.п.).
- Тест лежит в файле своей области поведения: `output_test.go`, `errors_test.go`, `selection_test.go`, `concurrency_test.go`, `usage_test.go`, `config_test.go`.
- Заготовки ответов GitLab — в `testdata/<проект>/` (реальные, урезанные до используемых полей) и `testdata/synthetic/` (ручные). Новая заготовка приходит в том же diff с записью в `testdata/README.md`.
- Параллельность доказывается синхронизацией внутри фейка (каналы, барьер: `reversedGlab`, `barrierGlab`), а не временем; из горутин фейка — `t.Error`, не `t.Fatal`.

## Защита от секретов

Репозиторий публичный, рядом с кодом живут пароль корпоративного прокси и хост корпоративного GitLab (ADR 0004). Три рубежа:
1. Pre-commit hook: gitleaks по индексу с правилами по умолчанию и своими правилами `.gitleaks.toml` (`url-credentials`, `gitlab-tree-url`), находки маскируются `--redact`.
2. `scripts/check-secrets.sh` из `check.sh`: случаи правил (`blocked`/`passes`), сквозные коммиты через hook в одноразовых репозиториях, скан всех файлов репозитория.
3. CI (`check.yml`): `gitleaks git` по всей истории коммитов — hook видит только текущий коммит.

Правила для агента:
- Hook — жёсткий барьер: никаких `--no-verify`, подмены `core.hooksPath`, `.gitleaksignore`, правок hook или `.gitleaks.toml` ради обхода.
- Нашёл секрет — остановись, сообщи файл, строку и `RuleID`, сам секрет не повторяй. Выдуманный пример переписывай на заглушки, которые правила пропускают (`<host>`, `example.com`, `$PROXY_PASSWORD`). В конфигах и тестах только выдуманные хосты (`corp.local`, `gitlab.example.com`), настоящие корпоративные домены запрещены даже в `.gitleaks.toml`.
- `gitleaks:allow` в строке или заглушка в `allowlist` правила — только на строку, которую нельзя переписать, и только после подтверждения пользователем.
- Правка `.gitleaks.toml` приходит в одном diff со случаями в `check-secrets.sh`: строка, которую правило ловит (`blocked`), и строка, которую пропускает (`passes`); флаг регистра в регулярке allowlist — на группу (`(?i:…)`), а не в начало выражения.

## CI/CD и релиз

- `.github/workflows/check.yml`: на push в `main` и на pull request — `check.sh` на Ubuntu и macOS (теми же `go install`) плюс отдельный job `secrets-history` со `gitleaks git` по полной истории.
- `.github/workflows/release.yml`: на тег `v*` — сначала тот же `check.sh`, затем сборка `CGO_ENABLED=0 go build -trimpath` для шести платформ (darwin/linux/windows × amd64/arm64) и `gh release create` с `checksums.txt` (SHA-256).
- Публикация релиза — по `release.sh`: `git checkout main && git pull && git tag v0.1.0 && git push origin v0.1.0`.

## Процесс работы над задачами

- Задачи лежат в `ai/<N>/`: постановка в `task.md`, затем `spec.md` и `ticket-<M>.md`. Спека вернее постановки.
- Ожидаемый процесс агента (по `AGENTS.md`): каждая задача начинается с `/grill-with-docs` (в `.claude/skills/`), затем spec → tickets → реализация через `/tdd` → `/review`. В `.claude/skills/` лежит набор инженерных skills с описанием в `.claude/skills/README.md`.
- Перед заявлением о готовности обязателен зелёный `scripts/check.sh`; Bash нужен для него и прогонов с `-race`, остальное (запуск, отладка, поиск вызовов) — через MCP GoLand по `.claude/rules/goland-mcp.md`, а при недоступном MCP — тем же Bash.
