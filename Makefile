SHELL := /bin/sh
.DEFAULT_GOAL := help

.PHONY: help setup install fmt fmt-check lint typecheck test test-integration test-e2e check build up down infra migrate dev-api dev-web commitlint

help:
	@echo 'setup / install    Create local environment / install dependencies'
	@echo 'up / down          Start / stop the complete Docker development stack'
	@echo 'infra              Start PostgreSQL and Mailpit for native development'
	@echo 'migrate            Apply PostgreSQL migrations'
	@echo 'dev-api / dev-web  Start a native development service'
	@echo 'check              Run formatting, lint, typecheck and unit tests'
	@echo 'test-integration   Run database tests using an isolated PostgreSQL container'
	@echo 'test-e2e           Run browser smoke tests against the running stack'

setup:
	node scripts/setup.mjs

install:
	npm ci
	cd apps/api && go mod download

fmt:
	gofmt -w apps/api
	npm run fmt

fmt-check:
	@test -z "$$(gofmt -l apps/api)" || { gofmt -l apps/api; exit 1; }
	npm run fmt:check

lint:
	cd apps/api && go vet ./...
	npm run lint

typecheck:
	cd apps/api && go build ./...
	npm run typecheck

test:
	cd apps/api && go test -race -count=1 ./...
	npm test

test-integration:
	./scripts/test-integration.sh

test-e2e:
	npm run test:e2e

check: fmt-check lint typecheck test

build:
	cd apps/api && go build -o bin/server ./cmd/server && go build -o bin/migrate ./cmd/migrate && go build -o bin/worker ./cmd/worker
	npm run build

up: setup
	docker compose up --build -d --wait --wait-timeout 180

down:
	docker compose down

infra: setup
	docker compose up -d --wait postgres mailpit

migrate:
	@set -a; . ./.env; set +a; cd apps/api && go run ./cmd/migrate

dev-api:
	@set -a; . ./.env; set +a; cd apps/api && go run ./cmd/server

dev-web:
	npm run dev

commitlint:
	npm run commitlint -- $(RANGE)

.PHONY: dev-worker
dev-worker:
	@set -a; . ./.env; set +a; cd apps/api && go run ./cmd/worker

.PHONY: test-recovery
test-recovery:
	node scripts/test-recovery.mjs
