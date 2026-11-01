.PHONY: test test-race run migrate tidy vet self-test

self-test:
	sh scripts/self-test.sh

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

tidy:
	go mod tidy

run:
	go run ./cmd/api

migrate:
	go run ./cmd/migrate
