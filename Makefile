.PHONY: build test test-integration vet lint fmt fmt-check tidy run \
	docker-build docker-up docker-down docker-reset docker-logs docker-preflight \
	migrate-up migrate-down smoketest docker-smoketest wait-ready

MIGRATIONS_DIR := migrations
LOCAL_DB_URL   := postgres://wordgame:wordgame@localhost:5432/wordgame?sslmode=disable

# Pin to a version confirmed to work against this module's go directive
# (see .golangci.yml) - bump deliberately, not via a floating `latest` tag.
GOLANGCI_LINT_VERSION := v2.13.1

# Prefer the Docker CLI/daemon; fall back to Podman if the docker socket
# isn't reachable (e.g. user not in the `docker` group). Override either way
# with `make <target> COMPOSE="docker compose"` or `COMPOSE="podman compose"`.
COMPOSE_BIN := $(shell docker ps >/dev/null 2>&1 && echo "docker compose" || echo "podman compose")
CONTAINER_BIN := $(shell docker ps >/dev/null 2>&1 && echo "docker" || echo "podman")

## Go

build: ## Build all packages
	go build ./...

test: ## Run unit tests (excludes the Docker-backed integration tests)
	go test ./...

test-integration: ## Run the integration tests for internal/store (requires Docker)
	go test -tags=integration ./internal/store/...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint via Docker/Podman (no host install required)
	# ./cmd/_smoketest is listed explicitly: golangci-lint follows go's own
	# convention of skipping underscore-prefixed dirs under ./..., same as
	# `go build`/`go vet` do - this repo still holds that package to the
	# same lint bar as everything else.
	$(CONTAINER_BIN) run --rm -v "$(CURDIR):/app" -w /app \
		golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) golangci-lint run ./... ./cmd/_smoketest

fmt: ## Reformat all Go source
	gofmt -w .

fmt-check: ## List files that need gofmt (non-empty output = failure)
	gofmt -l .

tidy: ## Sync go.mod/go.sum after dependency changes
	go mod tidy

run: ## Run the server against a local Postgres (APP_ENV=local, config.toml/secrets.yaml)
	APP_ENV=local go run ./cmd/wordgame

## Docker

docker-build: ## Build the wordgame image via docker/podman compose
	$(COMPOSE_BIN) build

docker-preflight: ## Tear down any stale containers from a previous run and flag port conflicts before starting
	@$(COMPOSE_BIN) down --remove-orphans >/dev/null 2>&1 || true
	@for port in 5432 1337; do \
		if (ss -ltn 2>/dev/null || netstat -ltn 2>/dev/null) | awk '{print $$4}' | grep -q ":$$port$$"; then \
			echo "warning: port $$port is already in use by something outside this project."; \
			echo "  find it with: sudo ss -ltnp | grep $$port   (or: sudo lsof -i :$$port)"; \
			echo "  then stop it before 'docker compose up' can bind that port."; \
		fi; \
	done

docker-up: docker-preflight ## Rebuild the image and start postgres + wordgame + swagger-ui in the background
	$(COMPOSE_BIN) up -d --build
	@echo "Swagger UI: http://localhost:8081"

docker-down: ## Stop postgres + wordgame
	$(COMPOSE_BIN) down

docker-reset: ## Stop everything and delete the Postgres volume
	$(COMPOSE_BIN) down -v

docker-logs: ## Follow the wordgame service's logs
	$(COMPOSE_BIN) logs -f wordgame

## Migrations (host-side, against a docker-compose-managed or other local Postgres;
## wordgame applies migrations itself on startup, see cmd/wordgame/main.go)

migrate-up: ## Apply all migrations to localhost:5432
	$(CONTAINER_BIN) run --rm --network host -v "$(CURDIR)/$(MIGRATIONS_DIR):/migrations" \
		migrate/migrate -path=/migrations -database "$(LOCAL_DB_URL)" up

migrate-down: ## Roll back one migration on localhost:5432
	$(CONTAINER_BIN) run --rm --network host -v "$(CURDIR)/$(MIGRATIONS_DIR):/migrations" \
		migrate/migrate -path=/migrations -database "$(LOCAL_DB_URL)" down 1

## Smoke test (cmd/_smoketest - a runnable, assertion-checked tour of the
## public API: auth, validation, a full game playthrough, and every error
## path, against a real running server + database)

smoketest: ## Run the smoke test against a server already listening on localhost:1337
	go run ./cmd/_smoketest

wait-ready: ## Block until the wordgame container's /readyz probe succeeds (used by docker-smoketest)
	@echo "waiting for http://localhost:1337/readyz..."
	@for i in $$(seq 1 30); do \
		curl -sf http://localhost:1337/readyz >/dev/null 2>&1 && exit 0; \
		sleep 1; \
	done; \
	echo "server did not become ready in time" >&2; exit 1

docker-smoketest: docker-up wait-ready smoketest ## Start the docker-compose stack, wait for it to be ready, then run the smoke test
