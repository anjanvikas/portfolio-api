SHELL := /bin/bash

BIN_DIR    := bin
BIN_NAME   := api
MIGRATIONS := migrations

# The Go binaries (cmd/api, cmd/seed) load .env themselves via
# config.LoadDotEnv, so we don't `include .env` from make — that would
# corrupt values containing `$` (e.g. bcrypt hashes for ADMIN_PASSWORD).
# The migrate target is the one exception that needs DATABASE_URL in its
# recipe scope; it sources .env inline.

.DEFAULT_GOAL := help

## help: list all targets
.PHONY: help
help:
	@echo "Targets:"
	@awk -F':' '/^## / { sub(/^## /, "", $$0); printf "  %s\n", $$0 }' $(MAKEFILE_LIST)

## tools: install dev tools (air, migrate, sqlc, golangci-lint)
.PHONY: tools
tools:
	go install github.com/air-verse/air@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	@command -v golangci-lint >/dev/null 2>&1 || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin

## sqlc: regenerate internal/store/*.sql.go from queries/ + migrations/
.PHONY: sqlc
sqlc:
	sqlc generate

## dev: run API with hot reload (Air)
.PHONY: dev
dev:
	air

## build: compile binary to ./bin/api
.PHONY: build
build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BIN_NAME) ./cmd/api

## run: build and run binary once (no hot reload)
.PHONY: run
run: build
	./$(BIN_DIR)/$(BIN_NAME)

## migrate: apply all pending migrations
.PHONY: migrate
migrate:
	@set -a && . ./.env && set +a && migrate -path $(MIGRATIONS) -database "$$DATABASE_URL" up

## migrate-down: roll back the most recent migration
.PHONY: migrate-down
migrate-down:
	@set -a && . ./.env && set +a && migrate -path $(MIGRATIONS) -database "$$DATABASE_URL" down 1

## migrate-new name=<slug>: create a new migration pair (up+down)
.PHONY: migrate-new
migrate-new:
	@if [ -z "$(name)" ]; then echo "usage: make migrate-new name=add_users_table"; exit 1; fi
	migrate create -ext sql -dir $(MIGRATIONS) -seq $(name)

## seed: seed local DB with sample data (idempotent)
.PHONY: seed
seed:
	go run ./cmd/seed

## hashpw: prompt for a passphrase and print a bcrypt hash for ADMIN_PASSWORD
.PHONY: hashpw
hashpw:
	@read -rsp "Passphrase: " pw && echo && printf %s "$$pw" | go run ./cmd/hashpw

## og-homepage: render + upload the static homepage OG image to R2 (one-shot)
.PHONY: og-homepage
og-homepage:
	go run ./cmd/gen-homepage-og

## lint: golangci-lint
.PHONY: lint
lint:
	golangci-lint run ./...

## test: go test
.PHONY: test
test:
	go test ./...

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -rf $(BIN_DIR) tmp
