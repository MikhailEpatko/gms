---
paths:
  - "**/*.go"
---

# GoLand MCP и gopls

Проект открыт в GoLand, его MCP-сервер `goland` проверяет файлы, запускает тесты, переименовывает символы и даёт терминал пользователя. Навигацию и дерево вызовов даёт LSP (gopls). `projectPath` в вызовах MCP — корень репозитория.

Если сервер `goland` недоступен, скажи об этом пользователю и работай через Bash: `go vet ./...` вместо `lint_files`, `go test ./internal/gms -run '^TestX$'` вместо run point.

## Проверка файлов

После каждой правки `.go`-файлов, до запуска тестов, прогони `lint_files` одним вызовом по всем изменённым `.go`-файлам. Шаг пройден, когда в ответе нет `ERROR`, а на изменённых строках нет новых `WARNING`. `Unhandled error` у `fmt.Fprint*` и `Set` флагов — фон, который уже есть в коде.

Компиляцию проверяет `lint_files`: `build_project` для Go диагностику не собирает.

## Тесты

Тесты запускает `execute_run_configuration` с `filePath` и `line`. Номера строк (run points) даёт `get_run_configurations(filePath)`:

- строка 1 — весь пакет `internal/gms`;
- строка `func Test…` — один тест;
- строка кейса в таблице — один подтест, самый быстрый цикл red-green.

Итог — `exitCode`. Вывод в формате `go test -json`; длинный обрезается, полный лог лежит в `fullOutputPath`, упавшие тесты ищи в нём по `"Action":"fail"`.

Run points запускают тесты без `-race` и `-count`. Финальная проверка — `scripts/check.sh` в Bash, многократный прогон с гонками — `go test -race -count=10 -run <Test> ./internal/gms` в Bash.

## Навигация и дерево вызовов

- Найти символ: LSP `workspaceSymbol` (имя, вид, файл, строка).
- Дерево вызовов: LSP `incomingCalls` (кто вызывает) и `outgoingCalls` (что вызывает) на позиции имени функции.
- Использования: LSP `findReferences`. Сигнатура и тип: LSP `hover` или MCP `get_symbol_info`.

MCP `analyze_calls` в GoLand 2026.2 не находит Go-функции ни по одной форме имени, а `search_symbol` возвращает координаты без имён (проверено 2026-10-30). После обновления GoLand проверь `analyze_calls` на `Run` из `internal/gms/run.go`, и если он заработал, поправь этот раздел.

## Отладка

В MCP GoLand 2026.2 нет инструментов отладчика: точки останова и пошаговое выполнение недоступны. Отладка идёт циклом `/diagnosing-bugs` на тех же инструментах:

- сигнал red — подтест, который падает на этом баге, запущенный по его run point;
- инструментирование — `t.Logf` в тесте или `fmt.Fprintf(os.Stderr, …)` в коде с меткой `[DEBUG-…]`; текст приходит в поле `Output` событий JSON;
- путь данных — LSP `incomingCalls`/`outgoingCalls` от места падения.

Настоящий `gms` против GitLab запускай через MCP `execute_terminal_command`: терминал IDE получает окружение пользователя, а Bash — окружение сессии Claude с её прокси.

## Правки

- Go-идентификатор переименовывает `rename_refactoring`: он обновляет все использования во всех файлах. Затем `lint_files` по файлам из `git diff --name-only`.
- Файлы создаёт Write и читает Read: `create_new_file` сразу добавляет файл в индекс git, а `read_file` обрезает середину длинного вывода.
