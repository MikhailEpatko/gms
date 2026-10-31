# Give Me Stats (gms)

CLI, который для каждого проекта и каждого окружения показывает текущий статус деплоя: дату деплоя, версию приложения и коммит деплоя.

## Установка

### Системные требования

- macOS, Linux или Windows (amd64 или arm64).
- [Go](https://go.dev/dl/) 1.27 или новее, нужен только для сборки.
- [glab](https://gitlab.com/gitlab-org/cli#installation), официальный CLI GitLab. `gms` вызывает его как подпроцесс, поэтому `glab` должен быть в `PATH` и залогинен в нужный GitLab с токеном не ниже `read_api`.
- Сетевой доступ к GitLab-инстансу из конфига.

Готовых бинарников нет, `gms` ставится через `go install` и собирается из исходников. Бинарник попадает в `$(go env GOPATH)/bin` (по умолчанию `~/go/bin`, в Windows `%USERPROFILE%\go\bin`).

### macOS

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

### Linux

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

### Windows

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
go install github.com/MikhailEpatko/nocomments@latest   # нужен check.sh и hook
scripts/check.sh   # gofmt, код без комментариев, vet, тесты, защита от секретов
go install .
```

Перед первым коммитом подключите pre-commit hook:

```bash
git config core.hooksPath .githooks
```

Hook сначала проверяет индекс на секреты (gitleaks с правилами из `.gitleaks.toml`) и не пустит коммит с секретом, затем запускает `scripts/check.sh` на копии индекса: проверяется ровно то, что попадёт в коммит, а незастейдженные правки не мешают. Без gitleaks и nocomments в `PATH` коммит тоже не пройдёт. `scripts/check.sh` через `scripts/check-secrets.sh` проверяет правила gitleaks на случаях, сам hook и все файлы репозитория. Push protection на GitHub — второй рубеж только для токенов известных провайдеров.

### Проверка

```bash
gms -c       # путь к конфигу, который будет прочитан
gms          # деплои всех проектов из конфига
```

## Запуск

- `gms` без флагов выводит все проекты из конфига.
- `gms -p project1,project3` выводит только перечисленные проекты (стандартный пакет `flag`). Пробелы вокруг кодов обрезаются, пустые элементы пропускаются, повторный `-p` накапливает коды, повторы схлопываются по первому вхождению. Позиционный аргумент (`gms -p project1 project3`) считается ошибкой с подсказкой `-p project1,project3`, exit 2.
- `gms -e DEV,DEMO` выводит у каждого проекта только перечисленные окружения, `gms -e PROD -p project1` выводит одно окружение одного проекта. Имена окружений сравниваются без учёта регистра, а в выводе пишутся как в конфиге. Значение разбирается по правилам `-p`, повторы имени схлопываются без учёта регистра, `-e` без единого имени считается ошибкой, exit 2. Проект без перечисленных окружений не выводится, в GitLab запрашиваются только выводимые пары. Имена через пробел (`gms -e DEV DEMO`) тоже считаются ошибкой, exit 2. Каждое лишнее слово относится к ближайшему `-p` или `-e` слева, а подсказка — полная исправленная команда: `gms -p project1 project3 -e PROD DEV` → `gms -p project1,project3 -e PROD,DEV`.
- `gms -c` печатает путь к конфигу и выходит, остальные флаги игнорируются.

Порядок вывода: проекты и окружения идут в том порядке, в каком записаны в файле конфига (JSON читается с сохранением порядка ключей, не через `map`). С `-p` проекты, а с `-e` окружения выводятся в порядке аргументов. Между окружениями ставятся 1 пустая строка и `-`×63, между проектами 2 пустые строки и `=`×63. Линия стоит вплотную к следующему заголовку, разделители только между блоками.

```
project1                                       // название проекта
DEV:                                           // окружение 
2026-09-24 16:35:33 MSK (MR !23)               // дата и время деплоя
7.0.2                                          // версия приложения (файл application.version в корне проекта со строкой "build_version=1.0.2")
1abfac4b Merge branch 'XXXXX-3488' into 'dev'  // хеш последнего задеплоенного коммита

---------------------------------------------------------------
DEMO:
нет деплоев


===============================================================
project2
...
```

- Дата выводится в локальной зоне, формат `2006-01-02 15:04:05 MST`. Пометка ставится, только если ref деплоя отличается от ветки окружения. Деплой из MR приходит с ref `refs/merge-requests/<N>/head` (или `.../merge` у merged results pipeline) и выводится как `(MR !<N>)`, любой другой чужой ref выводится как `(ветка: X)`.
- Коммит выводится как короткий SHA и заголовок.

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

- Неизвестный код в `-p`: ошибка до запросов со списком известных кодов, exit 2.
- Неизвестное имя в `-e`, то есть его нет ни у одного проекта конфига: ошибка до запросов со списком известных окружений всех проектов, exit 2. Неизвестные коды и имена выводятся одним сообщением, чтобы все опечатки исправлялись за один запуск.
- `-e` без единого имени: ошибка до запросов с примером `-e DEV,DEMO`, exit 2.
- Ни у одного проекта из `-p` нет ни одного окружения из `-e`: ошибка до запросов (`у проекта project2 нет окружения DEMO`), exit 2.
- `glab` не установлен или GitLab ответил анонимно (`currentUser: null`): выход без вывода в stdout с подсказкой `glab auth login --hostname gitlab.example.com`.
- У окружения нет деплоев или окружения нет в GitLab: строка `нет деплоев`.
- Проекта нет в GitLab или к нему нет доступа (GraphQL отвечает `null`), в деплое нет `commit.sha` или `job.finishedAt`: это ошибка пары. Нет `application.version` на коммите или пустое `build_version=`: это ошибка версии.
- Ошибка вызова `glab` (сеть или прокси, HTTP, в том числе 401 от истёкшего токена, ошибка GraphQL) становится ошибкой всех пар пачки с текстом из stderr `glab`, сведённым в одну строку без баннера `ERROR` и префикса `glab:`, без подсказки входа: для запроса деплоев это ошибка пары, для запроса версий это ошибка версии.
- Ошибка по паре проект+окружение выводится на месте этой пары, остальные пары выводятся, итоговый exit 1. Если не удалось получить только версию: строка `версия: ошибка (<причина>)`, дата и коммит выводятся.
