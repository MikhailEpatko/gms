# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Состояние проекта

`gms` — CLI, который для каждого проекта и каждого окружения показывает текущий деплой: дату, версию приложения и коммит. Модуль `github.com/MikhailEpatko/gms`, Go 1.27, сторонних зависимостей нет. Вся логика в `internal/gms` с точкой входа `Run(args, deps)`, `main.go` только подключает настоящий `glab`. Тесты разложены по областям поведения в `internal/gms/*_test.go`, общие хелперы и заготовки лежат в `helpers_test.go`. Отсутствие комментариев в коде проверяет внешняя программа `nocomments` (ADR 0005).

Задачи лежат в `ai/<N>/`, N — порядок выполнения: постановка в `task.md`, затем `spec.md` и `ticket-<M>.md`. Где спека расходится с `task.md`, верна спека. Поведение утилиты (флаги, порядок и формат вывода, конфиг, запросы к GitLab, ошибки) описано в `README.md`, разделы «Запуск»…«Ошибки», и README — источник правды. Долговременные решения лежат в `docs/adr/`, термины — в `GLOSSARY.md`.

## Команды

```bash
scripts/check.sh             # все проверки: gofmt, без комментариев (ADR 0005), vet, go test -race, защита от секретов; нужны gitleaks и nocomments; работа готова, только когда он прошёл
go build -o gms .            # сборка бинарника
go run . -p project1,project2         # запуск без сборки
go test ./internal/gms -run TestName -v   # один тест
```

Pre-commit hook из `.githooks/` сначала проверяет индекс на секреты (gitleaks с правилами `.gitleaks.toml` из корня), затем запускает тот же `scripts/check.sh` на копии индекса (ADR 0004), поэтому файл, нужный тестам, должен быть в индексе. В новом клоне hook подключается командой `git config core.hooksPath .githooks`, а gitleaks и nocomments ставятся по README.

Когда hook или `check.sh` нашли секрет, агент останавливается и сообщает пользователю файл, строку и `RuleID`, не повторяя сам секрет. Выдуманный пример в документе или тесте агент переписывает на заглушки, которые правила пропускают (`<host>`, `example.com`, `$PROXY_PASSWORD`), или описывает словами без полного URL. Исключение (`gitleaks:allow` в строке или заглушка в `allowlist` своего правила в `.gitleaks.toml`) ставится только на строку, которую так не переписать, и только после того, как пользователь подтвердил ложное срабатывание именно этой строки. Hook — жёсткий барьер: агент коммитит только через него, без `--no-verify`, подмены `core.hooksPath`, `.gitleaksignore` и правок hook или конфига ради обхода.

Агент проверяет файлы, запускает отдельные тесты, ищет вызовы, отлаживает и запускает настоящий `gms` через MCP GoLand и LSP по `.claude/rules/goland-mcp.md`, а Bash нужен для `scripts/check.sh` и прогонов с `-race`.

## Skills

В `.claude/skills/` лежит набор инженерных skills (tdd, review, to-spec, to-tickets, implement и др., описание в `.claude/skills/README.md`). Ожидаемый процесс: `/grill-with-docs` → spec → tickets → реализация через `/tdd` → `/review`.

Каждая задача, в том числе из `ai/<N>/task.md`, начинается с `/grill-with-docs`. Без него задача выполняется, только если пользователь явно так сказал.
