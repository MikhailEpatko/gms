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

Связи: golangci-lint из задачи 5 встраивается в `scripts/check.sh` и должен попасть в CI без отдельного шага.
Релизы из задачи 8 собираются в том же GitHub Actions.
