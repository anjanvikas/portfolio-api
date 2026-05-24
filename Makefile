SHELL := /bin/bash

BIN_DIR    := bin
BIN_NAME   := api
MIGRATIONS := migrations

# Load .env if present so make migrate / make dev pick up DATABASE_URL etc.
ifneq (,$(wildcard .env))
include .env
export
endif

.DEFAULT_GOAL := help

## help: list all targets
.PHONY: help
help:
	@echo "Targets:"
	@awk -F':' '/^## / { sub(/^## /, "", $$0); printf "  %s\n", $$0 }' $(MAKEFILE_LIST)

## tools: install dev tools (air, migrate, golangci-lint)
.PHONY: tools
tools:
	go install github.com/air-verse/air@latest
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@command -v golangci-lint >/dev/null 2>&1 || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin

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
	migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)" up

## migrate-down: roll back the most recent migration
.PHONY: migrate-down
migrate-down:
	migrate -path $(MIGRATIONS) -database "$(DATABASE_URL)" down 1

## migrate-new name=<slug>: create a new migration pair (up+down)
.PHONY: migrate-new
migrate-new:
	@if [ -z "$(name)" ]; then echo "usage: make migrate-new name=add_users_table"; exit 1; fi
	migrate create -ext sql -dir $(MIGRATIONS) -seq $(name)

## seed: seed local DB (no-op until entity stories land)
.PHONY: seed
seed:
	@echo "seed: no entities yet — implemented in F02 stories"

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
