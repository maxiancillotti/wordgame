# Testing wordgame

Three ways to exercise a running server, from quickest to most thorough:
swagger-ui (browser, zero setup), `curl` by hand (interactive), and
`cmd/_smoketest` (automated, assertion-checked). This doc covers all three —
for unit/integration test conventions see `CLAUDE.md`.

Every route requires three headers, since this service simulates sitting
behind an API gateway that has already authenticated the player rather than
handling login itself — see
`docs/adl/0003-transport-layer-design-decisions.md` for why:

| Header          | Value                                                  |
|------------------|--------------------------------------------------------|
| `Authorization`  | `Bearer maxkit-stub-service-token` (a fixed stub, not a real credential) |
| `X-User-Id`      | Any UUID — identifies the player. Two different UUIDs are two different players. |
| `X-User-Role`    | Always `client` (this service has no real role model)  |

## 1. Start the server

```sh
make docker-up   # Postgres + wordgame + swagger-ui, migrations + word seeding applied automatically
# - or -
make run         # against a local Postgres, APP_ENV=local (no swagger-ui without compose)
```

Either way, wordgame ends up listening on `http://localhost:1337`, with
`/livez`, `/readyz`, and `/metrics` available immediately and `/new`/`/guess`
available once migrations and word seeding finish (a few seconds).

## 2. swagger-ui — browse and call the API from a browser

`make docker-up` also starts a `swagger-ui` container serving
`docs/openapi.yaml` at **`http://localhost:8081`**, with a working "Try it
out" that calls the real running wordgame on `:1337` (CORS is wired
specifically for this — see `docs/adl/0003-*`).

1. Open `http://localhost:8081`.
2. Click **Authorize** (top right). The dialog shows three separate fields,
   one per header above (`bearerAuth`, `X-User-Id`, `X-User-Role` — not one
   combined bearer token, since this API's auth is three headers, not one).
   Fill in:
   - `bearerAuth`: `maxkit-stub-service-token`
   - `X-User-Id`: any UUID identifies a player — copy-paste
     `11111111-1111-1111-1111-111111111111`, or generate your own with
     `uuidgen` or `python3 -c "import uuid; print(uuid.uuid4())"`
   - `X-User-Role`: `client`

   Authorize, then close the dialog.
3. Expand `POST /new`, "Try it out", Execute. The response body is the new
   game's `id`, `current` board (all underscores), and `guesses_remaining`
   (starts at 6).
4. Expand `POST /guess`, "Try it out", fill in the request body with the
   `id` from step 3 and a single letter, Execute. Repeat with different
   letters to play the game out — the board reveals matched letters in
   place, and `guesses_remaining` only drops on a miss.
5. A guess after the game is won or lost (no `_` left, or
   `guesses_remaining` reaches 0) returns `409 Conflict`.

## 3. Interactively: `curl`

For scripting or a quick manual check without a browser. Real gameplay, so
which letters hit or miss depends on the actual (random) word chosen.

```sh
TOKEN="maxkit-stub-service-token"
USER_ID=$(python3 -c "import uuid; print(uuid.uuid4())")

# Start a game
curl -s -X POST http://localhost:1337/new \
  -H "Authorization: Bearer $TOKEN" -H "X-User-Id: $USER_ID" -H "X-User-Role: client"
# {"id":"...","current":"________","guesses_remaining":6}

GAME_ID="<id from the response above>"

# Guess a letter
curl -s -X POST http://localhost:1337/guess \
  -H "Authorization: Bearer $TOKEN" -H "X-User-Id: $USER_ID" -H "X-User-Role: client" \
  -d "{\"id\":\"$GAME_ID\",\"guess\":\"A\"}"
# {"id":"...","current":"______A_","guesses_remaining":6}  (hit)
# {"id":"...","current":"________","guesses_remaining":5}  (miss)
```

Error responses share one JSON shape (`internal/transport/rest` never rolls
its own):

```json
{"message":"game not found","code":"NOT_FOUND","traceId":"..."}
```

Worth trying by hand to see the error paths: omit a header (`401`), send a
2-character guess (`400`), guess on someone else's `id` with a different
`X-User-Id` (`403`), guess on a nonexistent `id` (`404`), keep guessing after
the game ends (`409`).

## 4. `cmd/_smoketest` — the automated tour

A standalone Go program (excluded from the normal build by its leading
underscore — see `CLAUDE.md`) that drives the running server over real HTTP
and asserts every response, doubling as a runnable reference for the whole
public API rather than just a happy-path ping. In order, it:

1. confirms missing auth, a wrong bearer token, and a missing `X-User-Role`
   are all rejected before ever reaching `/new`;
2. starts a game and asserts the initial board is all underscores with 6
   guesses remaining;
3. exercises guess validation (a two-character guess, a non-letter guess, a
   malformed game id), a guess against a nonexistent game (`404`), and a
   guess by a different `X-User-Id` against someone else's game (`403`);
4. plays the game out for real, one letter at a time through the alphabet,
   against the actual (unknown, randomly chosen) word — asserting after
   every guess that the board only ever grows more revealed (an
   already-revealed letter never changes) and that `guesses_remaining` only
   decrements by exactly 1 on a miss, never on a hit — until the game is won
   or lost;
5. confirms a guess against the now-completed game is rejected `409`;
6. confirms an unrouted path returns `404` with the same JSON error shape as
   every other failure.

If every assertion passes it logs `all assertions passed` and exits `0`; if
any status code or invariant doesn't match what that step expects, it logs
every mismatch it found and exits non-zero — safe to script against (CI,
evaluator checks) rather than needing to eyeball the log.

```sh
make smoketest          # go run ./cmd/_smoketest, expects something listening on :1337
```

Or, to bring up the whole Docker stack, wait for it to report ready, and run
the smoke test against it in one step:

```sh
make docker-smoketest
```
