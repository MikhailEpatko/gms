#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
	echo "gofmt: не отформатированы файлы (исправить: gofmt -w .):" >&2
	echo "$unformatted" >&2
	exit 1
fi
if ! command -v nocomments >/dev/null 2>&1; then
	echo "nocomments не найден, установите: go install github.com/MikhailEpatko/nocomments@latest" >&2
	exit 1
fi
nocomments
go vet ./...
go test -race -count=1 ./...
scripts/check-secrets.sh
