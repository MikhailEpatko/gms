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
if ! command -v golangci-lint >/dev/null 2>&1; then
	echo "golangci-lint не найден, установите: go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0" >&2
	exit 1
fi
go vet ./...
golangci-lint run
go test -race -count=1 ./...
scripts/check-secrets.sh
