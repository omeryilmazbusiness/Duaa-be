#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
echo "══ DUA-OS-BE self-test (cafe kernel) ══"
go vet ./...
go test ./...
go build -o /tmp/dua-os-api ./cmd/api
go build -o /tmp/dua-os-migrate ./cmd/migrate
sh scripts/live-chat-test.sh
echo "OK"
