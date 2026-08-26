# 2. Store layer (internal/store) design decisions

## Status

Accepted

## Context

This records the persistence-layer decisions made while building
`internal/store`: why persist to Postgres at all, how a random word is chosen
for a new game, and how the 370,102-word dictionary gets into the database.

## Decisions

### 1. Postgres, not an in-memory store, even though completed games can be discarded

The README allows clearing completed *games* from the store, but says nothing
about tolerating lost *in-progress* games. An in-memory store would drop every
active game on a crash or restart, mid-guess, with no way to recover it — the
player would simply lose their game state through no fault of their own. A
durable store avoids that regardless of how disposable a *finished* game's data
is. `words` also needs to live somewhere durable rather than be re-parsed from
`words.txt` on every boot, and reusing the same Postgres instance for both is
simpler than introducing a second storage mechanism just for one of them.

### 2. Random word selection: max-ID + point lookup, not OFFSET or COUNT(*)

`Storer.GetMaxWordID` returns the highest `id` in `words` (an aggregate that
Postgres rewrites into a backward index-only scan on the primary key, O(log N)),
and `Storer.GetWordByID` does a direct primary-key point lookup (also O(log N)).
The service layer picks `rand.Intn(maxID) + 1` and calls `GetWordByID` with it.

This assumes `words.id` is a contiguous `BIGSERIAL` sequence with no gaps — true
because it's populated by a single sequential seed pass (see decision 3 below)
with no deletes ever issued against the table.

**Rejected: `SELECT word FROM words ORDER BY random() LIMIT 1`.** Simple, but a
full-table scan assigning each row a random sort key — O(N) — on a 370k-row
table, on every single `/new` request.

**Rejected: `SELECT word FROM words ORDER BY id LIMIT 1 OFFSET n`.** Looks
index-backed, but Postgres still has to traverse and discard `n` rows before
returning the row at that position — O(offset), not O(log N).

**Rejected: `SELECT count(*) FROM words` to size the random range.**
`COUNT(*)` can't use an index-only scan under Postgres's MVCC visibility rules —
it's a full-table scan regardless of indexes. `GetMaxWordID`'s `coalesce(max(id),
0)` gets the same practical answer (an upper bound for the random pick) via
Postgres's documented MIN/MAX-over-indexed-column optimization instead.

### 3. `words` is seeded from `words.txt` in application code at startup, not a SQL migration

`words.txt` has 370,102 lines. A `golang-migrate` `.up.sql` migration has no
portable way to bulk-load a file that size:
- Literal `INSERT` statements for every row would be an unreviewable diff and
  slow to parse/apply as raw SQL text.
- Server-side `COPY words FROM '/path'` requires the *Postgres server process*
  to have filesystem access to that path — this breaks across environments
  (docker-compose, Heroku Postgres, any managed Postgres) and doesn't match how
  `words.txt` already ships alongside the *application* binary, not the
  database.

Instead, `seed.Words` (`seed/words.go`) runs from `cmd/wordgame/main.go` right
after `migrate.RunMigrations`, reading `words.txt` off the application's own
filesystem via the already-open `pgx` pool, batch-inserting with
`ON CONFLICT (word) DO NOTHING`.

**Rationale for running it on every boot rather than as a separate one-shot
step:** `ON CONFLICT DO NOTHING` makes it idempotent and safe to run
concurrently (a racing replica's duplicate insert is simply skipped, never an
error), so there's no exactly-once guarantee to protect — unlike a schema
migration, which is why `golang-migrate`'s version-tracking isn't needed here.
Running it automatically means a fresh environment (`docker-compose up`,
`go run ./cmd/wordgame`) just works, with no separate manual step an operator
could forget.
