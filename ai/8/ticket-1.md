# 1: Релизный workflow: check → build (6 целей) → публикация релиза с checksums

**What to build:** Файл `.github/workflows/release.yml` с триггером `push` тегов `v*` и тремя job'ами. `check` (ubuntu-latest): checkout → setup-go с `go-version-file: go.mod` → установка gitleaks `v8.30.1`, nocomments `@latest`, golangci-lint `v2.15.0` теми же командами, что в `check.yml` → `sh scripts/check.sh` целиком. `build` (`needs: check`): матрица `include` из 6 пар — darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64, windows/arm64; шаг `CGO_ENABLED=0 go build -trimpath -o gms-<goos>-<goarch>[.exe] .` (`.exe` только для windows); upload-artifact с бинарником. `release` (`needs: build`, `permissions: contents: write` — только у него): download-artifact в `dist/`, `shasum -a 256` по всем файлам в `checksums.txt`, `gh release create "$GITHUB_REF_NAME" --verify-tag --generate-notes` с файлами `dist/*`. Базовые `permissions: contents: read`, как в `check.yml`. Версия в бинарник не встраивается.

**Blocked by:** None (can start immediately).

**Status:** done

- [ ] Push тега с шаблоном `v*` запускает workflow, другие пушы — нет. _(проверяется на реальном теге, тикет 3)_
- [x] Локальная сборка всех 6 целей матрицы зелёная (`CGO_ENABLED=0 GOOS=… GOARCH=… go build -trimpath …`). _(Mach-O x86_64/arm64, ELF x86-64/arm64, PE32+ x86-64/Aarch64)_
- [x] В релизном workflow нет отдельных шагов gofmt/vet/test/сканов — единственный вызов проверок это `sh scripts/check.sh`, в job'е `build` нет ничего кроме сборки.
- [x] Матрица — ровно 6 пар из «Системных требований» README, имена артефактов `gms-<os>-<arch>` и `gms-windows-<arch>.exe`.
- [x] `contents: write` есть только у job'а `release`; у `check` и `build` — read.
- [ ] Job `release` создаёт релиз с тегом, автонотами и 7 файлами (6 бинарников + `checksums.txt`). _(проверяется на реальном теге, тикет 3)_
- [ ] Красная ветка: тег на коммите с ломающим gofmt изменением красит job `check`, релиз не публикуется. _(проверяется на реальном теге, тикет 3)_
