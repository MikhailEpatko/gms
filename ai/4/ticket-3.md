# 3: Защита ветки main — зелёные проверки обязательны (GitHub UI, вручную)

**What to build:** Настройка GitHub (не файл в репозитории): у ветки `main` включается branch protection — required status checks `check (ubuntu-latest)`, `check (macos-latest)`, `secrets-history` и запрет прямых push. После этого непроверенный коммит в `main` невозможен: ни через клон без hook, ни через веб-интерфейс. Выполняется один раз руками в Settings → Branches по приложенной пошаговой инструкции; включать после того, как оба предыдущих тикета дали зелёные прогоны, иначе GitHub не предложит имена чеков.

**Blocked by:** 1 (CI гоняет check.sh на push в main и на PR, бейдж в README), 2 (Скан всей истории gitleaks — job secrets-history) — нужны все три имени чеков и их зелёные прогоны.

**Status:** done

- [x] В required status checks включены ровно три чека: `check (ubuntu-latest)`, `check (macos-latest)`, `secrets-history`. _(включено через gh api, подтверждено ответом API)_
- [x] Прямые push в `main` запрещены, правило действует и для администраторов. _(enforce_admins: true; контрольный push шёл админ-токеном)_
- [x] Попытка прямого push в `main` отклоняется GitHub с сообщением про protection rules. _(GH006 «Changes must be made through a pull request», «3 of 3 required status checks are expected»)_
- [ ] Слияние PR, у которого хотя бы один из трёх чеков красный или не пришёл, блокируется. _(нужен красный тестовый PR — общий с красным критерием тикета 2)_
