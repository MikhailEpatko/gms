#!/bin/sh
# Все проверки репозитория: форматирование, vet, тесты с детектором гонок без кеша.
set -eu
cd "$(dirname "$0")/.."

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
	echo "gofmt: не отформатированы файлы (исправить: gofmt -w .):" >&2
	echo "$unformatted" >&2
	exit 1
fi
go vet ./...
go test -race -count=1 ./...
