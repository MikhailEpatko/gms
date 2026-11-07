# 2: Скан всей истории gitleaks — job secrets-history

**What to build:** Второй job workflow — `secrets-history` на `ubuntu-latest`: checkout с полной историей (`fetch-depth: 0`), тот же `go install` gitleaks, затем `gitleaks git --config .gitleaks.toml --redact -v .` по всей истории репозитория. Конфиг передаётся явно, как в `check-secrets.sh`, чтобы `GITLEAKS_CONFIG` ничего не подменял. Job ловит то, чего не видит hook и `check.sh`: секреты в коммитах, сделанных с `--no-verify` или через веб-интерфейс GitHub и пропущенные push protection (пароль в URL, корпоративный хост в URL ветки).

**Blocked by:** 1 (CI гоняет check.sh на push в main и на PR, бейдж в README) — тот же файл workflow и общий триггер.

**Status:** ready-for-agent

- [ ] Job `secrets-history` запускается на каждый push в `main` и на каждый PR вместе с `check` и зелёный на текущей истории (0 находок).
- [ ] В логе прогона видна команда скана с явно переданным конфигом репозитория.
- [ ] Красная ветка (на PR, не в `main`): временный коммит с секретом, который ловит правило `url-credentials`, красит job; в логе секрет замаскирован (`REDACTED` вместо значения, нет ни пароля, ни корпоративного хоста); после закрытия PR без слияния история `main` не изменилась.
