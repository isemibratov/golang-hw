BIN := ./bin/calendar
DOCKER_IMG="calendar:develop"
GOLANGCI_LINT_VERSION := 1.50.1
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

GIT_HASH := $(shell git log --format="%h" -n 1)
LDFLAGS := -X main.release="develop" -X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%S) -X main.gitHash=$(GIT_HASH)

build:
	mkdir -p $(dir $(BIN))
	go build -v -o $(BIN) -ldflags "$(LDFLAGS)" ./cmd/calendar

run: build
	$(BIN) --config=./configs/config.toml

build-img:
	docker build \
		--build-arg=LDFLAGS="$(LDFLAGS)" \
		-t $(DOCKER_IMG) \
		-f build/Dockerfile .

run-img: build-img
	docker run $(DOCKER_IMG)

version: build
	$(BIN) version

test:
	go test -v -count=1 -race -timeout=1m ./...

install-lint-deps:
	$(GOLANGCI_LINT) version 2>/dev/null | grep -Eq "version v?$(GOLANGCI_LINT_VERSION) " || \
		curl -sSfL https://golangci-lint.run/install.sh | \
		sh -s -- -b $(shell go env GOPATH)/bin v$(GOLANGCI_LINT_VERSION)

lint: install-lint-deps
	$(GOLANGCI_LINT) run ./...

migrate-up:
	@test -n "$(DATABASE_DSN)"
	@psql "$(DATABASE_DSN)" -v ON_ERROR_STOP=1 --single-transaction \
		-f migrations/00001_create_events.up.sql

migrate-down:
	@test -n "$(DATABASE_DSN)"
	@psql "$(DATABASE_DSN)" -v ON_ERROR_STOP=1 --single-transaction \
		-f migrations/00001_create_events.down.sql

migrate: migrate-up

.PHONY: build run build-img run-img version test install-lint-deps lint migrate migrate-up migrate-down
