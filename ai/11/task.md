Задача 11:

Выполнена.

Вынести программу проверки комментариев `scripts/nocomments` в самостоятельное приложение со своим репозиторием,
тестами и документацией, установить его и переключить `scripts/check.sh` на установленную программу.

Зачем: см. ADR 0005. Программу можно ставить в другие проекты, установленный бинарник запускается ~0,02 с вместо
~1,3 с у `go run` (и ~2,5 с в копии индекса из hook), а модуль gms содержит только gms.

Решено на `/grill-with-docs` (2026-10-31, Q1–Q8), spec и tickets не пишутся:
- Репозиторий `github.com/MikhailEpatko/nocomments`, публичный, локально `/Users/m.epatko/IdeaProjects/GO/nocomments`.
  `package main` в корне, `go.mod` с `go 1.22`, сторонних зависимостей нет.
- Поведение как у `scripts/nocomments`, без привязки к gms: те же виды файлов, пропуск `.git` и `.claude`, те же
  директивы, обход от текущего каталога, без флагов. Подсказка в конце без ссылки на ADR 0005.
- Сообщения, README и имена тестовых случаев на английском.
- Состав: `main.go`, `main_test.go` (seam `run(fs.FS, io.Writer)`, `fstest.MapFS`), `README.md`, `LICENSE` (MIT),
  `.gitignore`. CI нет.
- Тега нет, ставится `go install github.com/MikhailEpatko/nocomments@latest`.
- `scripts/check.sh` вызывает `nocomments` после `gofmt` и до `go vet`. Если программы нет в `PATH`, он печатает
  `nocomments не найден, установите: go install github.com/MikhailEpatko/nocomments@latest` и выходит с 1.
- ADR 0005 правится на месте: программа в отдельном репозитории, версия не закреплена, `go run ./scripts/nocomments`
  уходит в Considered Options.

Что ещё обновить в gms: удалить `scripts/nocomments/`; README «Сборка из клона» (установка рядом с gitleaks);
`AGENTS.md`; `CODING_STANDARDS.md` («Один seam», «Комментарии»); `ai/4/task.md` (CI ставит nocomments);
`ai/10/spec.md` (решение «пакет `main` в `scripts`» заменено задачей 11).

Связи: задача 10 (проверка комментариев), задача 4 (CI), ADR 0004 (hook запускает `check.sh` на копии индекса).
