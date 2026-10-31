#!/bin/bash
# Проверки защиты от секретов: правила .gitleaks.toml на случаях из ai/9/spec.md, pre-commit hook в одноразовых
# репозиториях и все файлы репозитория по текущим правилам. Запускается из scripts/check.sh, нужен gitleaks.
# Проверки независимы и идут параллельно; каждая печатает ok или FAIL в свой файл результатов.
set -u
cd "$(dirname "$0")/.."
root=$(pwd)

if ! command -v gitleaks >/dev/null 2>&1; then
	echo "gitleaks не найден, установите его по README.md" >&2
	exit 1
fi
# Из hook git передаёт переменные своего репозитория (GIT_INDEX_FILE и др.), а здесь git работает в чужих.
unset $(git rev-parse --local-env-vars)
unset GITLEAKS_CONFIG GITLEAKS_CONFIG_TOML

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
trap 'exit 1' INT TERM
mkdir -p "$work/res"

ok() { echo ok; }
bad() { printf 'FAIL %s\n' "$*"; }
expect() { if eval "$2"; then ok; else bad "$1"; fi; }

# run <проверка> [аргументы]: проверка в фоне, вывод — в файл результатов.
run() { "$@" >"$work/res/$1-${2:-}" 2>&1 & }

# --- Правила: каждая строка случая — отдельный файл, все файлы проверяются одним запуском gitleaks.
# Строки записаны с ~ вместо :// и собираются только во временном каталоге, поэтому сам скрипт правила проходят.
cases=$work/cases
mkdir -p "$cases"
names=()
rules=()
frags=()

# put <файл> <строка>: записывает строку, заменив ~ на ://.
put() { printf '%s\n' "${2//\~/://}" >"$1"; }

# blocked <имя> <правила> <строка> <фрагменты>: строку ловят все правила из списка, ни один фрагмент секрета
# не виден в выводе gitleaks. Файл случая назван номером, чтобы имя случая не совпало с фрагментом.
blocked() {
	put "$cases/${#names[@]}" "$3"
	names+=("$1")
	rules+=("$2")
	frags+=("$4")
}

# passes <имя> <строка>: у строки нет находок.
passes() { blocked "$1" "" "$2" ""; }

blocked 'pass внутри пароля' url-credentials 'export HTTPS_PROXY=http~ivanov:S3cr3tPassw0rd77@proxy.corp.local:3128' 'S3cr3tPassw0rd77 corp.local'
blocked 'логин user' url-credentials 'http~user:RealPassw0rd@proxy.corp.local:3128' 'RealPassw0rd corp.local'
blocked 'https в JSON' url-credentials '{"proxy": "https~svc:RealPassw0rd@db.corp.local/x"}' 'RealPassw0rd corp.local'
blocked 'похож на заглушку: passw0rd' url-credentials 'http~ivanov:passw0rd@proxy.corp.local:3128' 'passw0rd corp.local'
blocked 'example.com в начале хоста прокси' url-credentials 'http~ivanov:Qw7rty99@proxy.example.com.corp.local:3128' 'Qw7rty99 corp.local'
blocked 'хост notexample.com' url-credentials 'http~ivanov:Qw7rty99@notexample.com' 'Qw7rty99 notexample'
blocked 'пароль начинается с pass@' url-credentials 'http~ivanov:pass@word@proxy.corp.local:3128' 'word corp.local'
blocked '@ в пароле' url-credentials 'http~ivanov:P@ssw0rd@proxy.corp.local:3128' 'ssw0rd corp.local'
blocked 'апостроф в пароле' url-credentials "export HTTPS_PROXY=\"http~ivanov:it'sS3cret77@proxy.corp.local:3128\"" 'sS3cret77 corp.local'
blocked ': в пароле' url-credentials 'http~ivanov:S3c:r3t77@proxy.corp.local' 'r3t77 corp.local'
blocked '$ и строчные в пароле' url-credentials 'http~ivanov:$ecr3tPassw0rd@proxy.corp.local:3128' 'ecr3tPassw0rd corp.local'
blocked '$ и смешанный регистр' url-credentials 'http~user:$ProxyPass@proxy.corp.local:3128' 'ProxyPass corp.local'
blocked 'русский пароль с цифрами' url-credentials 'http~ivanov:пароль2024@proxy.corp.local:3128' 'пароль2024 corp.local'
blocked 'ссылка Markdown на прокси' url-credentials '[прокси](http~ivanov:Qw7rty99@proxy.corp.local)' 'Qw7rty99 corp.local'

passes 'заглушка pass' 'http~user:pass@proxy:3128'
passes 'заглушка password' 'http~user:password@proxy:3128'
passes 'заглушка PASSWORD' 'http~user:PASSWORD@proxy:3128'
passes 'заглушка $ИМЯ' 'http~user:$PROXY_PASSWORD@proxy:3128'
passes 'заглушка ${ИМЯ}' 'http~user:${PROXY_PASSWORD}@proxy:3128'
passes 'заглушка ${ИМЯ_2}' 'http~user:${PROXY_PASSWORD_2}@proxy:3128'
passes 'заглушка ${Имя}' 'http~user:${Proxy_Pass}@proxy.corp.local:3128'
passes 'заглушка <password>' 'http~user:<password>@proxy.corp.local:3128'
passes 'заглушка <пароль>' 'http~user:<пароль>@proxy.corp.local:3128'
passes 'маска REDACTED' 'Finding: export HTTPS_PROXY=http~ivanov:REDACTED@proxy.corp.local:3128'
passes 'заглушка логин:пароль@хост' 'http~логин:пароль@хост'
passes 'поддомен example.com у прокси' 'http~ivanov:pw@proxy.example.com:3128'
passes 'example.org у прокси' 'https~ivanov:pw@example.org/path'
passes 'Example.com в другом регистре' 'http~ivanov:pw@Proxy.Example.com:3128'
passes 'ссылка Markdown на example.com' '[прокси](http~ivanov:pw@proxy.example.com)'
passes 'URL в угловых скобках' '<http~ivanov:pw@proxy.example.com>'
passes 'URL в жирном шрифте' '**http~ivanov:pw@proxy.example.com**'
passes 'точка после URL' 'Пример: http~ivanov:pw@proxy.example.com.'
passes 'URL без пароля' 'http~127.0.0.1:10808 и ssh~git@gitlab.example.com:22/group/repo.git'
passes '@ в пути после порта' 'https~gitlab.example.com:8443/users/a@b'
passes 'пароль с gitleaks:allow' 'export HTTPS_PROXY=http~ivanov:S3cr3tPassw0rd77@proxy.corp.local:3128 # gitleaks:allow'

blocked 'конфиг JSON' gitlab-tree-url '{"p1": {"DEV": "https~gitlab.corp-internal.ru/grp/p1/-/tree/dev"}}' 'corp-internal'
blocked 'схема http' gitlab-tree-url 'http~gitlab.corp-internal.ru/grp/p1/-/tree/main' 'corp-internal'
blocked 'схема HTTPS заглавными' gitlab-tree-url 'HTTPS~gitlab.corp-internal.ru/grp/p1/-/tree/dev' 'corp-internal'
blocked 'хост начинается с gitlab.example.com' gitlab-tree-url 'https~gitlab.example.com.corp-internal.ru/grp/p1/-/tree/dev' 'corp-internal'
blocked 'хост mygitlab' gitlab-tree-url 'https~mygitlab/grp/p1/-/tree/dev' 'mygitlab'
blocked 'хост git.notexample.com' gitlab-tree-url 'https~git.notexample.com/grp/p1/-/tree/dev' 'notexample'
blocked 'хост с портом' gitlab-tree-url 'https~gitlab.corp-internal.ru:8443/grp/p1/-/tree/dev' 'corp-internal'
blocked 'ссылка Markdown на ветку' gitlab-tree-url '[DEV](https~gitlab.corp-internal.ru/grp/p1/-/tree/dev)' 'corp-internal'
blocked 'вложенные группы' gitlab-tree-url 'https~git.corp-internal.ru/a/b/c/p1/-/tree/feature/x' 'corp-internal'
blocked 'логин в URL ветки' gitlab-tree-url 'https~user@gitlab.corp-internal.ru/grp/p1/-/tree/dev' 'corp-internal'
blocked 'путь к репозиторию скрыт' gitlab-tree-url 'https~gitlab.corp-internal.ru/secretgroup/p1/-/tree/dev' 'secretgroup'
blocked 'пароль и хост в одном URL' 'gitlab-tree-url url-credentials' 'https~ivanov:Qw7rty99@gitlab.corp-internal.ru/grp/p1/-/tree/dev' 'corp-internal Qw7rty99'

passes 'gitlab.example.com в конфиге' '"DEV": "https~gitlab.example.com/group/project1/-/tree/dev"'
passes 'gitlab.example.org' 'https~gitlab.example.org/group/project1/-/tree/dev'
passes 'поддомен example.com у ветки' 'https~git.example.com/group/project1/-/tree/dev'
passes 'example.org без поддомена' 'https~example.org/group/project1/-/tree/dev'
passes 'gitlab.com' 'https~gitlab.com/group/project1/-/tree/main'
passes 'HTTPS и GITLAB.COM заглавными' 'HTTPS~GITLAB.COM/group/project1/-/tree/main'
passes 'хост gitlab' 'https~gitlab/project1/-/tree/dev'
passes 'заглушка <host>' 'https~<host>/<group>/<repo>/-/tree/<branch>'
passes 'заглушка <gitlab-host>' 'https~<gitlab-host>/<group>/<repo>/-/tree/<branch>'
passes 'GitLab.Example.com' 'https~GitLab.Example.com/group/project1/-/tree/dev'
passes 'example.com с портом' 'https~gitlab.example.com:8443/group/project1/-/tree/dev'
passes 'логин на example.com' 'https~user@gitlab.example.com/group/project1/-/tree/dev'
passes 'ссылка Markdown на example.com у ветки' '[DEV](https~gitlab.example.com/group/project1/-/tree/dev)'
passes 'URL ветки с gitleaks:allow' '{"DEV": "https~gitlab.corp-internal.ru/grp/p1/-/tree/dev"} // gitleaks:allow'
passes 'ссылка на MR' 'https~gitlab.corp-internal.ru/grp/p1/-/merge_requests/23'
passes 'ссылка на pipeline' 'https~gitlab.corp-internal.ru/grp/p1/-/pipelines/100500'
passes 'ссылка на blob' 'https~gitlab.corp-internal.ru/grp/p1/-/blob/main/README.md'

# leaks <каталог> <путь>: gitleaks с правилами репозитория и выводом как у hook (-v --redact), путь — от каталога.
leaks() { (cd "$1" && gitleaks dir --no-banner --no-color --redact -v --config "$root/.gitleaks.toml" "$2" 2>&1); }

# Вывод gitleaks разбит на блоки находок по файлам случаев.
check_rules() {
	local out=$work/rules.out i rule frag found
	leaks "$work" cases >"$out"
	awk -v dir="$work/found" 'BEGIN { RS = "" } { for (i = 1; i < NF; i++) if ($i == "File:" && $(i + 1) ~ /^cases\//) { f = $(i + 1); sub(/^cases\//, "", f); print $0 "\n" >> (dir "/" f) } }' "$out"
	set -f
	for i in "${!names[@]}"; do
		found=$(cat "$work/found/$i" 2>/dev/null)
		if [ -z "${rules[$i]}" ]; then
			expect "правила: ${names[$i]} проходит, а найдено:"$'\n'"$found" '[ -z "$found" ]'
			continue
		fi
		for rule in ${rules[$i]}; do
			expect "правила: ${names[$i]} ловит $rule" 'printf "%s\n" "$found" | grep -qE "^RuleID: +$rule\$"'
		done
		for frag in ${frags[$i]}; do
			expect "правила: ${names[$i]} скрывает $frag" '! printf "%s" "$found" | grep -qF -- "$frag"'
		done
	done
}
mkdir -p "$work/found"
run check_rules

# --- Файлы репозитория проходят текущие правила: в клоне — отслеживаемые и новые, в копии индекса из hook,
# где нет .git, — все. Запускается до подмены глобального конфига git ниже: ls-files учитывает
# core.excludesFile пользователя. .gitleaksignore не копируется, чтобы не пропустить находки молча.
check_files() {
	local files=$work/files paths=() f scan rc
	mkdir -p "$files"
	if [ -e .git ]; then
		list() { git ls-files -z -co --exclude-standard; }
	else
		list() { find . -type f -print0; }
	fi
	while IFS= read -r -d '' f; do
		f=${f#./}
		[ -f "$f" ] && [ "${f##*/}" != .gitleaksignore ] && paths+=("$f")
	done < <(list)
	tar -cf - "${paths[@]}" | tar -xf - -C "$files"
	scan=$(leaks "$files" .)
	rc=$?
	expect "файлы репозитория проходят правила:"$'\n'"$scan" '[ $rc -eq 0 ]'
}
run check_files

# --- Hook: коммиты в копиях базового репозитория с hook и .gitleaks.toml из этого репозитория. Заглушка
# check.sh печатает CHECK-RAN и падает, если в её копии репозитория файл state содержит red. Запуск процесса
# на рабочей машине дорогой (ADR 0004), поэтому совместимые случаи собраны в один коммит, а проверки результата
# обходятся встроенными командами bash.

# Свой глобальный конфиг git: автор коммитов есть и в CI, а hooksPath, подпись и шаблоны пользователя не мешают.
export GIT_CONFIG_GLOBAL=$work/gitconfig GIT_CONFIG_NOSYSTEM=1
printf '[user]\n\tname = gms\n\temail = gms@example.com\n[commit]\n\tgpgsign = false\n[core]\n\thooksPath = .githooks\n[init]\n\tdefaultBranch = main\n' >"$GIT_CONFIG_GLOBAL"

base=$work/base
mkdir -p "$base/.githooks" "$base/scripts"
cp "$root/.githooks/pre-commit" "$base/.githooks/"
cp "$root/.gitleaks.toml" "$base/"
cat >"$base/scripts/check.sh" <<'EOF'
#!/bin/sh
echo CHECK-RAN
{ read -r state <"${0%/*}/../state"; } 2>/dev/null
[ "${state:-}" != red ]
EOF
chmod +x "$base/scripts/check.sh" "$base/.githooks/pre-commit"
git -C "$base" init -q
git -C "$base" add -A
git -C "$base" -c core.hooksPath=/dev/null commit -q -m base

# PATH без gitleaks: в нём только git, которому hook и нужен до проверки gitleaks.
mkdir -p "$work/nogl"
ln -s "$(command -v git)" "$work/nogl/git"

# token: TOK — случайный токен в формате правила gitleaks gitlab-pat.
token() {
	local abc=ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789 i
	TOK=glpat-
	for i in {1..20}; do TOK+=${abc:RANDOM%62:1}; done
}

# stage_token: TOK — новый токен, записан в a.txt и добавлен в индекс R.
stage_token() {
	token
	put "$R/a.txt" "token = \"$TOK\""
	git -C "$R" add a.txt
}

# fresh <имя>: R — копия базового репозитория, T — пустой TMPDIR для hook, чтобы проверить, что hook его убрал.
fresh() {
	R=$work/run/$1
	T=$work/tmp/$1
	mkdir -p "$work/run" "$T"
	cp -R "$base" "$R"
}

# try_commit [PATH]: коммит индекса R. OUT — вывод, RC — код, NEW — создан ли коммит.
try_commit() {
	local before after
	read -r before <"$R/.git/refs/heads/main"
	OUT=$(cd "$R" && TMPDIR=$T PATH=${1:-$PATH} git commit -m case 2>&1)
	RC=$?
	read -r after <"$R/.git/refs/heads/main"
	if [ "$after" != "$before" ]; then NEW=yes; else NEW=no; fi
}

has() { [[ $OUT == *"$1"* ]]; }
cleaned() { ! compgen -G "$T/*" >/dev/null; }

# hook_case <имя>: один сквозной случай hook в своей копии базового репозитория.
hook_case() {
	fresh "$1"
	case $1 in
	findings)
		stage_token
		put "$R/b.txt" 'proxy = http~ivanov:S3cr3tPassw0rd77@proxy.corp.local:3128'
		git -C "$R" add b.txt
		try_commit
		expect "hook: секреты в индексе — коммит не создан, check.sh не запускался" '[ $NEW = no ] && [ $RC -ne 0 ] && ! has CHECK-RAN'
		expect "hook: ловят правила по умолчанию и свои из .gitleaks.toml" 'has gitlab-pat && has url-credentials'
		expect "hook: в выводе File и Line" 'has "File:" && has "Line:"'
		expect "hook: секреты замаскированы" 'has REDACTED && ! has "$TOK" && ! has S3cr3tPassw0rd77'
		expect "hook: подсказка после находки" 'has "коммит остановлен: в индексе найден секрет" && has "gitleaks:allow"'
		;;
	env-config)
		put "$T.toml" "[[rules]]"$'\n'"id = \"never\""$'\n'"regex = '''NEVER_MATCH_q7z9'''"
		put "$R/a.txt" 'proxy = http~ivanov:S3cr3tPassw0rd77@proxy.corp.local:3128'
		git -C "$R" add a.txt
		OUT=$(cd "$R" && TMPDIR=$T GITLEAKS_CONFIG=$T.toml git commit -m case 2>&1)
		expect "hook: GITLEAKS_CONFIG не подменяет правила" 'has url-credentials'
		;;
	staged-secret)
		stage_token
		put "$R/a.txt" clean
		try_commit
		expect "hook: секрет в индексе ловится, даже если в рабочем дереве его уже нет" '[ $NEW = no ] && has gitlab-pat'
		;;
	missing)
		put "$R/clean.txt" clean
		git -C "$R" add clean.txt
		try_commit "$work/nogl"
		expect "hook: без gitleaks — коммит не создан, команда установки" '[ $NEW = no ] && [ $RC -eq 1 ] && ! has CHECK-RAN && has "gitleaks не найден, установите: go install github.com/zricethezav/gitleaks/v8@v8.30.1"'
		;;
	broken-config)
		put "$R/.gitleaks.toml" '[extend'
		git -C "$R" add .gitleaks.toml
		try_commit
		expect "hook: битый .gitleaks.toml — коммит не создан" '[ $NEW = no ] && ! has CHECK-RAN'
		;;
	ignore-file)
		stage_token
		put "$R/.gitleaksignore" 'a.txt:gitlab-pat:1'
		try_commit
		expect "hook: .gitleaksignore — отказ, секрет не пропущен" '[ $NEW = no ] && [ $RC -eq 1 ] && has "удалите .gitleaksignore" && ! has CHECK-RAN'
		git -C "$R" rm -q --cached a.txt
		put "$R/clean.txt" clean
		git -C "$R" add clean.txt
		try_commit
		expect "hook: .gitleaksignore — отказ и без секрета в индексе" '[ $NEW = no ] && has "удалите .gitleaksignore" && ! has CHECK-RAN'
		;;
	dirty-worktree)
		put "$R/state" green
		git -C "$R" add state
		put "$R/state" red
		token
		put "$R/untracked.txt" "token = \"$TOK\""
		try_commit
		expect "hook: секрет и красная правка только в рабочем дереве не мешают коммиту" '[ $NEW = yes ] && has CHECK-RAN'
		expect "hook: временная копия индекса убрана" cleaned
		;;
	red-index)
		put "$R/state" red
		git -C "$R" add state
		put "$R/state" green
		try_commit
		expect "hook: красный check.sh на индексе останавливает коммит, даже если рабочее дерево исправлено" '[ $NEW = no ] && has CHECK-RAN'
		expect "hook: временная копия индекса убрана и после красного check.sh" cleaned
		;;
	esac
}
for c in findings env-config staged-secret missing broken-config ignore-file dirty-worktree red-index; do
	run hook_case "$c"
done

wait
results=$(cat "$work"/res/*)
passed=$(printf '%s\n' "$results" | grep -c '^ok$')
failed=$(printf '%s\n' "$results" | grep -c '^FAIL')
printf '%s\n' "$results" | grep -v '^ok$' >&2
if [ "$failed" -ne 0 ]; then
	echo "check-secrets: $failed проверок не прошли, $passed прошли" >&2
	exit 1
fi
echo "check-secrets: $passed проверок прошли"
