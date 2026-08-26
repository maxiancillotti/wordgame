# 1. Business layer (internal/service) design decisions

## Status

Accepted

## Context

This records the game-rule decisions the README leaves unspecified, made while
building out `internal/service`.

## Decisions

### 1. Repeat guesses are not tracked

No guessed-letter history is stored. A guess counts as a "hit" if the letter is
anywhere in the word (idempotent — re-revealing an already-revealed letter costs
nothing) and as a "miss" otherwise, **every time** — a repeated wrong guess
decrements `guesses_remaining` again.

**Rationale:** matches the README's literal per-guess rule ("the player's guess
matches a letter" / "does not match") and avoids an extra schema column or
data structure just to remember which letters were already tried.

### 2. Guessing on a completed game is rejected

A game is "completed" once `guesses_remaining` reaches 0 or every character is
revealed. Guessing on a completed game returns `apperr.StateConflict` (HTTP 409)
rather than silently no-op'ing or re-evaluating the guess.

**Rationale:** the README describes the game ending but never describes what
happens if a client keeps guessing afterward; erroring is safer than silently
accepting a guess that can no longer change anything.

### 3. A game can only be guessed on by the user who created it

`Servicer.GuessLetter` takes the caller's `userID` (from `X-User-Id`, see ADR 3)
and compares it against `models.Game.UserID`; a mismatch returns
`apperr.Forbidden` (HTTP 403), checked before the completed-game check above so a
non-owner never learns a game's state.

**Rationale:** the README doesn't specify per-user isolation, but `POST /new`
already scopes a game to a `userID`, so allowing any caller who knows a game's
UUID to guess on someone else's game would be an unintentional gap, not a
deliberate feature.

### 4. Word selection is a business-layer decision over a store-layer primitive

The store only knows how to answer "what's the highest word ID" and "give me the
word with this ID" — picking *which* ID (the randomness) happens in
`internal/service`, not `internal/store`. See ADR 2 for why.
