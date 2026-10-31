Задача 4:

Нужно запускать `scripts/check.sh` в GitHub Actions на каждый push в `main` и на каждый pull request.

Зачем: сейчас проверки запускает только pre-commit hook из `.githooks/`, а его в каждом клоне нужно включать вручную
(`git config core.hooksPath .githooks`). Коммит без hook или правка через веб-интерфейс GitHub попадают в `main`
непроверенными, а `go install github.com/MikhailEpatko/gms@latest` ставит пользователям ровно то, что лежит в `main`.

Что известно:
- Репозиторий `github.com/MikhailEpatko/gms` публичный, каталога `.github/` и workflows в нём нет.
- Версия Go задана в `go.mod` (`go 1.27`).
- Тесты не обращаются к сети и используют зону `Europe/Moscow`.
- README обещает macOS, Linux и Windows. Но тесты пути к конфигу в `internal/gms/usage_test.go` ждут литерал
  `/home/dev/.config/gms/config.json`, а `configPath` собирает путь через `filepath.Join`, так что на Windows они,
  судя по коду, упадут. `scripts/check.sh` написан на `sh`.

Открытые вопросы:
- На каких ОС запускать: только Linux или матрица с macOS и Windows. Если с Windows, то что делать с тестами путей.
- Нужен ли бейдж статуса в README.
- Требовать ли зелёные проверки перед слиянием в `main` (это настройка GitHub, а не файл в репозитории).
- Как сканировать историю gitleaks с конфигом `.gitleaks.toml` из задачи 9. CI опаздывает (секрет уже публичный),
  но ловит коммиты с `--no-verify` и правки через веб-интерфейс GitHub, которые pre-commit hook не видит, а push
  protection GitHub пропускает (пароль в URL, URL ветки с настоящим хостом). Вероятно, хватит готового бинарника
  релиза gitleaks v8.30.1 и `gitleaks git` по пушнутому диапазону или всей истории (0,7 с).

Связи: golangci-lint из задачи 5 встраивается в `scripts/check.sh` и должен попасть в CI без отдельного шага.
Релизы из задачи 8 собираются в том же GitHub Actions. `scripts/check.sh` вызывает `scripts/check-secrets.sh`
(ADR 0004) и `nocomments` (ADR 0005), поэтому в CI нужны gitleaks и nocomments в `PATH`. Скрипт проверяет
правила, hook и текущие файлы, но не историю: `gitleaks git` по истории остаётся отдельным шагом CI.
`check-secrets.sh` написан на `bash`.
