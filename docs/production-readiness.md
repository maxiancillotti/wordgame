# Production readiness

This is a coding-challenge submission, not a production service — nothing
below is implemented. This records what would actually need to happen before
treating it as one, organized the way the gap was framed: infrastructure,
delivery pipeline, and business/API surface.

## 1. Kubernetes deployment

Today, `docker-compose.yml` is local-only: it builds the image, bakes
`docker/config.toml`/`docker/secrets.yaml` into it, and runs a single
Postgres container. None of that is a production shape.

- **Deployment + Service (+ Ingress or Gateway)**, replacing `docker-compose.yml`.
  `app.Serve` already exposes `/livez` and `/readyz` (maxkit) — wire those
  directly as the Deployment's liveness/readiness probes rather than adding
  new ones.
- **Config via ConfigMap, secrets via Secret** (or an external secrets
  operator/Vault), not baked into the image. `docker/config.toml`'s comments
  already say the credentials "intentionally do NOT go here" and
  `docker/secrets.yaml` is explicitly documented as
  local-placeholder-only — that split is the right shape, it just needs a
  real secrets backend behind it instead of a file copied into the image at
  build time.
- **Managed Postgres** (RDS/Cloud SQL/etc.), not a Postgres container — the
  in-cluster `postgres:16-alpine` container in `docker-compose.yml` is a dev
  convenience, not something to run stateful in a cluster without a real
  operator (e.g. CloudNativePG) backing it, and a managed instance sidesteps
  that entirely for a service this size.
- **Migration/seed startup behavior needs re-examination at replica count > 1.**
  `cmd/wordgame/main.go` currently runs `migrate.RunMigrations` and
  `seed.Words` inline on every boot. `seed.Words` is safe under concurrent
  replicas by design (`ON CONFLICT DO NOTHING`, see
  `docs/adl/0002-store-layer-design-decisions.md`); `golang-migrate`'s
  Postgres driver takes an advisory lock so concurrent migration attempts
  don't race either — but running a 370k-row seed pass as part of every
  pod's readiness path is still wasteful at scale (each replica after the
  first pays the cost of scanning the table to find nothing to insert). A
  Kubernetes `Job`/init container running migrations+seed once before the
  Deployment rolls out would be the more standard shape.
- **Resource requests/limits and an HPA** — no resource profiling has been
  done at all yet; this needs load data before numbers mean anything.

## 2. CI/CD (GitHub Actions)

There is no pipeline at all right now — everything (`make lint`, `make test`,
`make test-integration`, `make docker-smoketest`) is run by hand, as
documented in `docs/testing.md` and `CLAUDE.md`. A real pipeline would run
the same targets this repo already has, on every PR:

1. `make lint` (golangci-lint — already containerized, see `.golangci.yml`).
2. `make test` (unit tests).
3. `make test-integration` (store integration tests — GitHub Actions runners
   have Docker available natively, so the existing testcontainers-based
   setup in `internal/store/main_integration_test.go` needs no changes).
4. `make docker-smoketest` (full end-to-end: build the image, bring up
   Postgres + the app, run `cmd/_smoketest` against it for real) as a final
   gate — this is the one stage that would catch a regression none of the
   others would (e.g. a Dockerfile or docker-compose wiring mistake).
5. Build and push the image to a registry (GHCR) on merge to `main` and on
   tags, then that's the artifact the Kubernetes manifests in §1 reference —
   not `docker compose build`'s local tag.
6. `govulncheck ./...` for known CVEs in dependencies, as a separate check
   from `gosec` (which `make lint` already runs, but only flags code
   patterns, not vulnerable dependency versions).

Branch protection requiring 1–4 (and 6) to pass before merge is the actual
enforcement mechanism — a pipeline that runs but isn't required gets ignored.

## 3. Business/API surface

- **Persist individual guesses, not just current board state.** Right now
  `games.guessed_currently`/`games.guesses_remaining` (see
  `migrations/000001_init_schema.up.sql`) only capture the *result* of every
  guess ever made, not the guesses themselves — there's no way to answer
  "what did this player actually guess, and in what order." A `guesses`
  table (`game_id` FK, `letter`, `hit boolean`, `guessed_at`) would capture
  that, and enables a read method (`Servicer.ListGuesses`/`GET
  /games/{id}/guesses` or similar) to retrieve the history — useful for
  replay, an activity feed, or basic analytics (average guesses per win,
  most-missed letters, etc.), none of which the current schema can answer.
- **Add a `status` field to the game-state response**
  (`internal/transport/rest/dto.go`'s `GameStateDTO`) — `"ongoing"`,
  `"won"`, or `"lost"` — so a client knows the outcome immediately instead
  of inferring it from `guesses_remaining == 0` or the absence of `_` in
  `current`. The business logic for this already exists internally
  (`internal/service/service.go`'s `isCompleted`, plus the win/loss
  distinction it doesn't currently need to make); it just isn't surfaced
  through `models.Game`/the DTO today. This is a small, self-contained
  change relative to the other items here.
- Worth considering alongside the two above, though not asked for directly:
  a `GET /games/{id}` read endpoint (there's currently no way to fetch a
  game's state without guessing), which would also be the natural place to
  return `status` and the guess history together.
