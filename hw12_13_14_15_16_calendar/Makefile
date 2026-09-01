BIN_DIR := ./bin
BIN := $(BIN_DIR)/calendar
SCHEDULER_BIN := $(BIN_DIR)/calendar_scheduler
STORER_BIN := $(BIN_DIR)/calendar_storer
GOLANGCI_LINT_VERSION := 1.50.1
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint
OAPI_CODEGEN_VERSION := v1.12.4
OAPI_CODEGEN := go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)
OPENAPI_SPEC := ./api/swagger.json
OPENAPI_GENERATED := ./internal/server/http/openapi/api.gen.go
COMPOSE_PROJECT_NAME ?= calendar
INTEGRATION_COMPOSE_PROJECT_NAME ?= calendar-integration
COMPOSE := docker compose --project-name $(COMPOSE_PROJECT_NAME) \
	--file deployments/docker-compose.yaml
INTEGRATION_COMPOSE := docker compose --project-name $(INTEGRATION_COMPOSE_PROJECT_NAME) \
	--file deployments/docker-compose.yaml \
	--file deployments/docker-compose.integration.yaml

GIT_HASH := $(shell git log --format="%h" -n 1)
LDFLAGS := -X main.release="develop" -X main.buildDate=$(shell date -u +%Y-%m-%dT%H:%M:%S) -X main.gitHash=$(GIT_HASH)
export LDFLAGS

build:
	mkdir -p $(BIN_DIR)
	go build -v -o $(BIN) -ldflags "$(LDFLAGS)" ./cmd/calendar
	go build -v -o $(SCHEDULER_BIN) ./cmd/calendar_scheduler
	go build -v -o $(STORER_BIN) ./cmd/calendar_storer

run: build
	$(BIN) --config=./configs/config.toml

run-scheduler: build
	$(SCHEDULER_BIN) --config=./configs/scheduler.toml

run-storer: build
	$(STORER_BIN) --config=./configs/storer.toml

generate:
	$(OAPI_CODEGEN) -generate types,chi-server -package openapi \
		-o $(OPENAPI_GENERATED) $(OPENAPI_SPEC)

generate-check: generate
	git diff --exit-code -- $(OPENAPI_GENERATED)

build-img:
	$(COMPOSE) build calendar scheduler storer

run-img: up

up:
	$(COMPOSE) up --detach --build --wait

down:
	$(COMPOSE) down --remove-orphans

integration-tests:
	@set -u; \
	status=0; \
	cleanup() { $(INTEGRATION_COMPOSE) down --volumes --remove-orphans; }; \
	trap 'status=130; cleanup; exit $$status' HUP INT TERM; \
	cleanup >/dev/null 2>&1 || true; \
	$(INTEGRATION_COMPOSE) build integration-tests calendar scheduler storer || status=$$?; \
	if [ $$status -eq 0 ]; then \
		$(INTEGRATION_COMPOSE) up --detach --wait calendar scheduler storer || status=$$?; \
	fi; \
	if [ $$status -eq 0 ]; then \
		$(INTEGRATION_COMPOSE) run --rm --no-deps integration-tests || status=$$?; \
	fi; \
	if [ $$status -ne 0 ]; then \
		$(INTEGRATION_COMPOSE) ps --all; \
		$(INTEGRATION_COMPOSE) logs --no-color; \
	fi; \
	cleanup || { cleanup_status=$$?; [ $$status -ne 0 ] || status=$$cleanup_status; }; \
	exit $$status

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
		-f migrations/00001_create_events.up.sql \
		-f migrations/00002_notifications.up.sql

migrate-down:
	@test -n "$(DATABASE_DSN)"
	@psql "$(DATABASE_DSN)" -v ON_ERROR_STOP=1 --single-transaction \
		-f migrations/00002_notifications.down.sql \
		-f migrations/00001_create_events.down.sql

migrate: migrate-up

.PHONY: build run run-scheduler run-storer generate generate-check build-img run-img up down \
	integration-tests version test install-lint-deps lint migrate migrate-up migrate-down
