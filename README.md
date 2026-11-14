# Give Me Stats (gms)

[![check](https://github.com/MikhailEpatko/gms/actions/workflows/check.yml/badge.svg)](https://github.com/MikhailEpatko/gms/actions/workflows/check.yml)

CLI, который для каждого проекта и каждого окружения показывает текущий статус деплоя: дату деплоя, версию приложения и коммит деплоя.

## Установка

### Системные требования

- macOS, Linux или Windows (amd64 или arm64).
- [glab](https://gitlab.com/gitlab-org/cli#installation), официальный CLI GitLab. `gms` вызывает его как подпроцесс, поэтому `glab` должен быть в `PATH` и залогинен в нужный GitLab с токеном не ниже `read_api`.
- Сетевой доступ к GitLab-инстансу из конфига.

[Go](https://go.dev/dl/) 1.27 или новее нужен только для установки из исходников — разделы «Из исходников (`go install`)» и «Сборка из клона». Для готового бинарника из релиза Go не требуется.

### Из релизов

Готовые бинарники для всех платформ из «Системных требований» публикуются в [GitHub Releases](https://github.com/MikhailEpatko/gms/releases/latest). Скачайте из последнего релиза файл под свою платформу и `checksums.txt`:

| Платформа | Файл |
|---|---|
| macOS Intel | `gms-darwin-amd64` |
| macOS Apple Silicon | `gms-darwin-arm64` |
| Linux amd64 | `gms-linux-amd64` |
| Linux arm64 | `gms-linux-arm64` |
| Windows amd64 | `gms-windows-amd64.exe` |
| Windows arm64 | `gms-windows-arm64.exe` |

`checksums.txt` содержит SHA-256 всех файлов релиза. Положите скачанные файлы в один каталог и проверьте:

```bash
shasum -a 256 -c checksums.txt   # macOS, Linux
```

```powershell
Get-FileHash gms-windows-amd64.exe   # Windows: сравните хеш со строкой файла в checksums.txt
```

macOS и Linux — сделать бинарник исполняемым и положить в каталог из `PATH` (пример с `/usr/local/bin`, подставьте свой файл и каталог):

```bash
chmod +x gms-darwin-arm64
sudo mv gms-darwin-arm64 /usr/local/bin/gms
```

Windows (PowerShell) — положить `gms-windows-amd64.exe` в каталог из `PATH` под именем `gms.exe`.

Бинарники релиза не подписаны. macOS помечает файл, скачанный браузером, карантином `com.apple.quarantine`, и Gatekeeper может не пустить его при первом запуске: снимите карантин командой `xattr -d com.apple.quarantine /usr/local/bin/gms` либо разрешите запуск в «Системных настройках → Конфиденциальность и безопасность».

Дальше как при любой установке: `glab auth login --hostname gitlab.example.com`, затем конфиг `~/.config/gms/config.json` — `gms` без конфига напечатает пример (раздел «Конфиг»).

### Из исходников (`go install`)

`go install` собирает `gms` из исходников и кладёт бинарник в `$(go env GOPATH)/bin` (по умолчанию `~/go/bin`, в Windows `%USERPROFILE%\go\bin`) — убедитесь, что каталог есть в `PATH`.

#### macOS

```bash
# 1. Зависимости
brew install go glab
# если brew не ставит glab (например, Intel-мак на свежей macOS), соберите его из исходников:
# go install gitlab.com/gitlab-org/cli/cmd/glab@latest

# 2. Авторизация в GitLab
glab auth login --hostname gitlab.example.com

# 3. Установка gms
go install github.com/MikhailEpatko/gms@latest

# 4. Убедитесь, что $(go env GOPATH)/bin есть в PATH (для zsh)
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc && source ~/.zshrc

# 5. Конфиг
mkdir -p ~/.config/gms
gms          # без конфига напечатает пример, сохраните его в ~/.config/gms/config.json
```

#### Linux

```bash
# 1. Зависимости
# Go 1.27+ берите с https://go.dev/dl/: в репозиториях дистрибутивов версия часто старее
sudo rm -rf /usr/local/go
curl -LO https://go.dev/dl/go1.27.0.linux-amd64.tar.gz   # подставьте актуальную версию и архитектуру
sudo tar -C /usr/local -xzf go1.27.0.linux-amd64.tar.gz
echo 'export PATH="$PATH:/usr/local/go/bin:$HOME/go/bin"' >> ~/.bashrc && source ~/.bashrc

# glab: пакет дистрибутива, snap или Homebrew; если ни одного нет, соберите из исходников:
go install gitlab.com/gitlab-org/cli/cmd/glab@latest

# 2. Авторизация в GitLab
glab auth login --hostname gitlab.example.com

# 3. Установка gms
go install github.com/MikhailEpatko/gms@latest

# 4. Конфиг
mkdir -p ~/.config/gms
gms          # без конфига напечатает пример, сохраните его в ~/.config/gms/config.json
```

#### Windows

Команды для PowerShell.

```powershell
# 1. Зависимости
winget install GoLang.Go glab.glab
# перезапустите PowerShell, чтобы обновился PATH (установщик Go добавляет в него %USERPROFILE%\go\bin)

# 2. Авторизация в GitLab
glab auth login --hostname gitlab.example.com

# 3. Установка gms
go install github.com/MikhailEpatko/gms@latest

# 4. Конфиг
New-Item -ItemType Directory -Force "$env:USERPROFILE\.config\gms" | Out-Null
gms          # без конфига напечатает пример, сохраните его в %USERPROFILE%\.config\gms\config.json
```

Если задан `HOME` (например, в Git Bash), конфиг ищется в `$HOME/.config/gms/config.json`, как на macOS и Linux.

### Сборка из клона

```bash
git clone https://github.com/MikhailEpatko/gms.git
cd gms
go install github.com/zricethezav/gitleaks/v8@v8.30.1   # или brew install gitleaks: нужен check.sh и hook
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0   # или brew install golangci-lint: нужен check.sh и hook
go install github.com/MikhailEpatko/nocomments@latest   # нужен check.sh и hook
scripts/check.sh   # gofmt, код без комментариев, vet, линтер, тесты, защита от секретов
go install .
```

Перед первым коммитом подключите pre-commit hook:

```bash
git config core.hooksPath .githooks
```

Hook сначала проверяет индекс на секреты (gitleaks с правилами из `.gitleaks.toml`) и не пустит коммит с секретом, затем запускает `scripts/check.sh` на копии индекса: проверяется ровно то, что попадёт в коммит, а незастейдженные правки не мешают. Без gitleaks, nocomments и golangci-lint в `PATH` коммит тоже не пройдёт. `scripts/check.sh` через `scripts/check-secrets.sh` проверяет правила gitleaks на случаях, сам hook и все файлы репозитория. Push protection на GitHub — второй рубеж только для токенов известных провайдеров.

Те же проверки гоняет GitHub Actions: `scripts/check.sh` запускается на каждый push в `main` и каждый pull request на Ubuntu и macOS; статус — бейдж вверху README. Gitleaks, nocomments и golangci-lint CI ставит теми же командами `go install`. Отдельный job сканирует всю историю коммитов `gitleaks git`'ом: hook и `check.sh` видят только текущий коммит, а секрет в старом — например, от коммита с `--no-verify` или правки через веб-интерфейс — ловится CI.

### Проверка

```bash
gms -c       # {"config": "путь к конфигу, который будет прочитан"}
gms          # отчёт по деплоям всех проектов из конфига
```

## Запуск

- `gms` без флагов выводит все проекты из конфига.
- `gms -p project1,project3` выводит только перечисленные проекты (стандартный пакет `flag`). Пробелы вокруг кодов обрезаются, пустые элементы пропускаются, повторный `-p` накапливает коды, повторы схлопываются по первому вхождению. Позиционный аргумент (`gms -p project1 project3`) считается ошибкой с подсказкой `-p project1,project3`, exit 2.
- `gms -e DEV,DEMO` выводит у каждого проекта только перечисленные окружения, `gms -e PROD -p project1` выводит одно окружение одного проекта. Имена окружений сравниваются без учёта регистра, а в выводе пишутся как в конфиге. Значение разбирается по правилам `-p`, повторы имени схлопываются без учёта регистра, `-e` без единого имени считается ошибкой, exit 2. Проект без перечисленных окружений не выводится, в GitLab запрашиваются только выводимые пары. Имена через пробел (`gms -e DEV DEMO`) тоже считаются ошибкой, exit 2. Каждое лишнее слово относится к ближайшему `-p` или `-e` слева, а подсказка — полная исправленная команда: `gms -p project1 project3 -e PROD DEV` → `gms -p project1,project3 -e PROD,DEV`.
- `gms -c` печатает путь к конфигу и выходит, остальные флаги игнорируются.

Весь вывод — JSON. Отчёт по парам — массив объектов с отступами в два пробела, один объект на пару. Порядок объектов — порядок вывода: как в файле конфига (JSON читается с сохранением порядка ключей, не через `map`), а с `-p` и `-e` — в порядке аргументов:

```json
[
  {
    "project": "project2",
    "environment": "DEV",
    "deployed_at": "2026-09-24T16:35:33+03:00",
    "version": "7.0.2",
    "commit": "1abfac43",
    "title": "Merge branch 'XXXXX-3488' into 'dev'"
  },
  {
    "project": "project2",
    "environment": "DEMO",
    "error": "Нет деплоев"
  }
]
```

- Ключи объекта пары: `project` (код проекта) и `environment` (имя окружения как в конфиге) — всегда; при наличии деплоя — `deployed_at` (RFC 3339, локальная зона), `version`, `commit` (короткий SHA) и `title` (заголовок коммита целиком, без обрезки). Известное заполнено, неизвестное опущено: у окружения без деплоев объект состоит из `project`, `environment` и `error`, при ошибке версии `version` отсутствует, а остальные поля деплоя на месте.
- Поле `error` — состояние пары: строка `Нет деплоев` или причина сбоя. `Нет деплоев` — не ошибка: если все пары ответили, а у части нет деплоев, код выхода `0`. Сбой пары или версии читается по коду выхода `1`.
- Версия — значение `build_version` из файла `application.version` в корне проекта на коммите деплоя. Ветка деплоя в отчёт не передаётся: сверять, что окружение развёрнуто из своей ветки, — ответственность потребителя.
- Разбор через `jq`: `gms | jq -r '.[] | [.project, .environment, .version] | @tsv'` — таблица версий, `gms | jq -r '.[] | select(.environment == "PROD") | .version'` — версии всех PROD, `gms | jq -r '.[] | select(.error != null and .deployed_at == null) | "\(.project) \(.environment)"'` — пары с ошибками.

Служебный вывод тоже JSON. `gms -h` печатает в stdout `{"usage": "…"}`, `gms -c` — `{"config": "…"}`. Ошибки использования и выполнения печатаются в stderr объектом с `error` и опциональными полями: `hint` (что сделать), `known` (список известных кодов или имён), `path` и `example` (путь и пример конфига), `usage` (текст справки при ошибках разбора флагов). Отчёт, справка и путь идут в stdout, ошибки — в stderr; при недоступном `glab` stdout остаётся пустым.

### Конфиг

Путь: `~/.config/gms/config.json`, переопределяется через env `GMS_CONFIG`. Домашний каталог берётся из `HOME`, а если он не задан, из `os.UserHomeDir()` (в Windows это `%USERPROFILE%`). Если не удалось определить ни `GMS_CONFIG`, ни домашний каталог, это ошибка, exit 1: путь не строится относительно текущего каталога.
Конфиг - это JSON вида `{ "<код проекта>": { "<ОКРУЖЕНИЕ>": "<url ветки>" } }`, URL вида `https://gitlab.example.com/group/<repo>/-/tree/<branch>`. 
Из URL извлекаются хост, путь проекта в GitLab и ветка окружения. Окружения произвольные, берутся из ключей. 
Ошибка в конфиге — exit 1 с сообщением `некорректный конфиг <путь>: …`: в ошибке значения указаны проект и окружение (`project2.DEV: …`), в синтаксической ошибке JSON — номер строки (`<путь>:4: …`).
Если файла конфига нет, `gms` печатает «создайте ~/.config/gms/config.json» и пример конфига с обезличенными URL (`gitlab.example.com`), exit ≠ 0.

### Откуда берутся данные

Все запросы выполняются через подпроцесс `glab api --hostname <host> graphql` (ADR 0001). Токенов в `gms` нет. Пары одного хоста запрашиваются пачками до 8 пар, пачки идут одновременно. На пачку приходится два запуска `glab`: деплои и версии (ADR 0003).

1. **Текущий деплой** (ADR 0002, 0003): `project(fullPath).environment(name: <имя окружения в нижнем регистре>).lastDeployment(status: SUCCESS)`. Ветка не фильтруется. Дата деплоя = `job.finishedAt`, коммит = `commit.shortId` и `commit.title`. Тем же запросом читается `currentUser`: `null` означает, что `glab` не залогинен. Совпадение с Deployments API проверено вживую на всех 12 парах 2026-10-18.
2. **Версия приложения**: строка `build_version=` из `application.version` **на коммите деплоя** (`repository.blobs(ref: <sha>)`), а не на HEAD ветки.

### Ошибки

- Неизвестный код в `-p`: ошибка до запросов с кодом `2`, в stderr `{"error": "неизвестный код проекта: foo", "known": ["project2", "project1"]}`.
- Неизвестное имя в `-e`, то есть его нет ни у одного проекта конфига: ошибка до запросов со списком известных окружений всех проектов, exit 2. Неизвестные коды и имена выводятся двумя объектами подряд, чтобы все опечатки исправлялись за один запуск.
- `-e` без единого имени: ошибка до запросов с примером `{"error": "в -e не указано ни одно окружение", "hint": "пример: -e DEV,DEMO"}`, exit 2.
- Ни у одного проекта из `-p` нет ни одного окружения из `-e`: ошибка до запросов (`{"error": "у проекта project2 нет окружения DEMO"}`), exit 2.
- `glab` не установлен или GitLab ответил анонимно (`currentUser: null`): выход без вывода в stdout, в stderr `{"error": "…", "hint": "выполните: glab auth login --hostname gitlab.example.com"}`, exit 1.
- У окружения нет деплоев или окружения нет в GitLab: в объекте пары `"error": "Нет деплоев"`, exit не меняется.
- Проекта нет в GitLab или к нему нет доступа (GraphQL отвечает `null`), в деплое нет `commit.sha` или `job.finishedAt`: в объекте пары `error` с причиной, exit 1. Нет `application.version` на коммите или пустое `build_version=`: причина в `error`, а дата и коммит пары на месте, exit 1.
- Ошибка вызова `glab` (сеть или прокси, HTTP, в том числе 401 от истёкшего токена, ошибка GraphQL) становится `error` всех пар пачки с текстом из stderr `glab`, сведённым в одну строку без баннера `ERROR` и префикса `glab:`, без подсказки входа.
- Ошибка в конфиге — exit 1 с объектом `{"error": "некорректный конфиг <путь>: …"}`: в ошибке значения указаны проект и окружение (`project2.DEV: …`), в синтаксической ошибке JSON — номер строки (`<путь>:4: …`).
- Если файла конфига нет, `gms` печатает в stderr `{"error": "нет файла конфига", "path": "…", "example": "…"}` с примером конфига с обезличенными URL (`gitlab.example.com`), exit 1.
