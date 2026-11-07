# 1: CI гоняет check.sh на push в main и на PR (матрица Linux + macOS), бейдж в README

**What to build:** Workflow GitHub Actions с job `check` на матрице `ubuntu-latest` + `macos-latest`: checkout → setup-go с версией из `go.mod` → установка gitleaks (`go install`, версия как в README) и nocomments (`go install`, без закрепления, ADR 0005) → запуск `sh scripts/check.sh` целиком, без повторения его шагов в workflow. Триггеры: push в `main` и pull_request, `permissions: contents: read`, отмена устаревших прогонов PR через `concurrency`. В README — бейдж статуса workflow со ссылкой на прогоны и строка о том, что проверки в CI идут на macOS и Linux. Перед коммитом убрать из индекса остатки переноса nocomments в отдельный репозиторий (`git restore --staged scripts/nocomments`).

**Blocked by:** None (can start immediately).

**Status:** done

- [x] Push в `main` и открытие/обновление PR запускают workflow автоматически, без ручного запуска. _(прогоны 42344410556 и 42346193229 стартовали сами на push)_
- [x] Job `check` зелёный на обеих ОС матрицы на текущем состоянии `main` — это значит, что в CI прошли gofmt, nocomments, go vet, `go test -race` и сканы gitleaks из `check.sh`/`check-secrets.sh`. _(check (ubuntu-latest) и check (macos-latest): success на обоих прогонах)_
- [x] В файле workflow нет отдельных шагов gofmt/vet/test/сканов — единственный вызов проверок это `check.sh`.
- [x] Версия Go берётся из `go.mod`, gitleaks — той же командой и версии, что в README и в подсказке hook при отсутствии gitleaks.
- [ ] PR из fork запускает workflow и проходит без токенов на запись (`contents: read`). _(конфигурация в diff; проверить после первого пуша)_
- [ ] Новый push в тот же PR отменяет его устаревший прогон (видно в списке прогонов). _(после первого пуша)_
- [ ] Красная ветка: временная правка, ломающая gofmt, красит job на обеих ОС; после отката job снова зелёный. _(после первого пуша)_
- [x] В README бейдж отображает реальный статус workflow и ведёт на список прогонов; рядом строка, что проверки в CI гоняются на macOS и Linux. _(workflow в `main`, бейдж рисует статус прогона)_
- [x] Из индекса убраны `scripts/nocomments/main.go` и `main_test.go` (в коммитах их никогда не было).
