# Project Overview

wordgame is a small word-guessing (hangman-style) HTTP API: `POST /new` starts a
game against a randomly chosen word, `POST /guess` evaluates one letter against
an in-progress game. See `README.md` for the full game rules and API contract.
Design decisions the README leaves unspecified (repeat guesses, guessing on a
completed game, per-user game ownership, word selection/seeding, the auth model)
are recorded in `docs/adl/`.

## Commands

```sh
# Run against a local Postgres (config.toml/secrets.yaml)
make run

# Run the full stack (Postgres + app + swagger-ui) via Docker
make docker-up   # prints the Swagger UI URL (http://localhost:8081)
make docker-down
make docker-reset   # also drops the Postgres volume

# Test
make test               # unit tests
make test-integration   # store integration tests, requires Docker
make docker-smoketest   # full end-to-end run: docker-up + wait-ready + smoketest
make smoketest          # smoke test only, against a server already listening on :1337

# Lint / format
go vet ./...
make fmt-check
make lint   # golangci-lint via Docker/Podman, no host install required

# Migrations (wordgame also applies these itself at startup)
make migrate-up
make migrate-down
```

## Architecture

Layered: transport → business logic → persistence, following this repo's
`service-builder`/`store-builder` skill conventions.

- `cmd/wordgame/main.go` — entry point. Uses maxkit's `app.Init`/`app.Serve` for
  bootstrapping, config, logging, and the standard middleware chain (auth,
  tracing, metrics, panic recovery). Runs migrations and seeds `words` before
  serving.
- `internal/service` — business rules (word selection, guess evaluation,
  win/loss, completed-game/ownership checks). Declares the `Storer` port
  (consumer-defined interface) that `internal/store` implements — the service
  layer never imports `internal/store` directly.
- `internal/store` — Postgres implementation of `Storer`, via `pgx/v5`.
- `internal/transport/rest` — plain `net/http` handlers/router; the player's
  identity comes from `X-User-Id` (already resolved by an upstream
  auth/gateway layer this service trusts — see `docs/adl/0003-*`).
- `internal/models` — domain types (`Game`, `Word`) shared across layers.
- `internal/config` — service-specific config on top of maxkit's
  `InfrastructureConfig`.
- `seed/` — bulk-loads `words.txt` into the `words` table at startup
  (idempotent via `ON CONFLICT DO NOTHING`); see `docs/adl/0002-*` for why
  this isn't a SQL migration.
- `migrations/` — schema-only SQL migrations, applied automatically by
  `migrate.RunMigrations` at boot.
- `cmd/_smoketest` — standalone runnable end-to-end check against a live
  server (auth, validation, a full game playthrough, every error path).
  Leading underscore keeps it out of `go build ./...`; run via
  `make smoketest`/`make docker-smoketest`.
- `docs/openapi.yaml` — served by the `swagger-ui` container in
  `docker-compose.yml` (`make docker-up` prints its URL) for browser-based
  "Try it out". Requires `app.WithCORS` (wired in `cmd/wordgame/main.go`) and
  `docker/config.toml`'s `[infrastructure.cors]` block, which explicitly
  lists `X-User-Id`/`X-User-Role` in `allowed_headers` - maxkit's CORS
  default only allows `Authorization`/`Content-Type`.
- `docs/testing.md` — the reviewer-facing guide to exercising the running
  API: swagger-ui, a `curl` walkthrough of every error path, and what
  `cmd/_smoketest` checks. Start here when asked how to test this repo.
- `docs/production-readiness.md` — this repo is a coding-challenge
  submission, not a production service; this records what would actually
  need to happen before treating it as one (Kubernetes, CI/CD, a Grafana
  dashboard for the already-instrumented RED metrics, guess-history
  persistence, a `status` field). Nothing in it is implemented.

## Folder Structure

```
cmd/wordgame/            entrypoint (app wiring, HTTP handler build)
cmd/_smoketest/          standalone end-to-end check against a live server
internal/service/       business logic + Storer port + sentinels
internal/store/          Postgres implementation of Storer
internal/transport/rest/ HTTP handlers, router, DTOs
internal/models/        domain types (Game, Word)
internal/config/        service config
seed/                    words.txt -> words table loader
migrations/              schema migrations (golang-migrate format)
docs/adl/                architecture decision records
docs/openapi.yaml        API spec, served by the swagger-ui compose service
docs/testing.md          reviewer guide: swagger-ui, curl, cmd/_smoketest
docs/production-readiness.md  what's left before this could be "prod"
.golangci.yml            lint config (make lint)
```

## Tech Stack

- Go 1.26
- REST API over stdlib `net/http`
- PostgreSQL via `pgx/v5`
- `gitlab.com/maxi.ancillotti/maxkit` — app bootstrapping (`app.Init`/
  `app.Serve`), `apperr` (typed errors → HTTP status), Postgres pool/migration
  helpers, structured logging (zap), tracing/metrics
- Docker / docker-compose for local Postgres + the app itself

## Conventions

- New code should pass `make lint` (`.golangci.yml`) clean, same as
  `go vet`/`gofmt` — including `cmd/_smoketest`, which golangci-lint would
  otherwise skip like `go build`/`go vet` do (underscore-prefixed dirs are
  excluded from `./...`), so `make lint` names it explicitly. A finding that's
  a genuine false positive for this codebase gets a scoped `//nolint:<linter>`
  with a one-line reason, not a blanket rule disable.
- The `Storer` interface is declared in `internal/service` (consumer-defined),
  implemented in `internal/store` — preserve this inversion; don't have the
  service layer import store types.
- Sentinel errors (`service.ErrNotFound`) are declared by the business layer;
  `internal/store` translates driver errors into them. `apperr` (HTTP-status
  mapping) is only used in `internal/service` and `internal/transport` — never
  in `internal/store`.
- Business rule validation goes through a single `validateXxx(args) error`
  entry point (`internal/service/validate.go`), not scattered inline checks.
- Prefer hand-rolled fakes over mocks for tests (`fake_store_test.go`,
  `fake_servicer_test.go`), using `testify`'s `require`/`assert`.
- Store-layer integration tests are build-tagged `integration` and spin up a
  real Postgres via testcontainers (see `internal/store/main_integration_test.go`).

## Testing

- Unit tests live alongside the file they test (`_test.go`), using
  hand-rolled fakes for the layer's own dependency (fake `Storer` for
  `internal/service`, fake `Servicer` for `internal/transport/rest`).
- `internal/store/*_integration_test.go` are `//go:build integration` and
  require Docker; run via `make test-integration`.
- `cmd/_smoketest` is a plain Go program, not a `go test` suite (testify's
  `assert.TestingT` is satisfied here by a small reporter that logs via
  maxkit's zap logger instead of failing a `*testing.T`) — run it against a
  live server, it's not part of `go test ./...`.
- New business rules should be covered in `internal/service` against the fake
  store before anything else — that's where win/loss/conflict/ownership logic
  actually lives.
