// Command _smoketest exercises a running wordgame server end to end against
// real HTTP: start a game, play it out letter by letter against the real
// (unknown, randomly chosen) word until it's won or lost, and assert every
// response along the way - board-state monotonicity, guesses-remaining
// bookkeeping, and every error path (missing auth, bad input, wrong owner,
// completed game, unrouted path). It's meant as a runnable tour of the
// public API for anyone evaluating this repo, not just a happy-path check:
// every step asserts the HTTP status (and, where relevant, response shape)
// it expects, and the process exits non-zero with a summary of what failed
// the first time one doesn't match, rather than only printing responses.
//
// Not part of the build (the leading underscore makes the go tool skip this
// directory) - run it explicitly with `make smoketest` (against a server
// already started via `make run` or `make docker-up`), or directly:
//
//	APP_ENV=local go run ./cmd/wordgame   # in one shell
//	go run ./cmd/_smoketest               # in another
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"gitlab.com/maxi.ancillotti/maxkit/pkg/obs/logging"
)

const (
	baseURL = "http://localhost:1337"

	// The stub service-to-service token maxkit's default auth middleware
	// checks (gitlab.com/maxi.ancillotti/maxkit/pkg/app.stubServiceToken).
	// See docs/adl/0003-transport-layer-design-decisions.md for why every
	// request needs this plus X-User-Id/X-User-Role.
	stubServiceToken = "maxkit-stub-service-token" //nolint:gosec // a public, documented stub token hardcoded in maxkit itself, not a real credential
)

// gameState mirrors internal/transport/rest.GameStateDTO.
type gameState struct {
	ID               string `json:"id"`
	Current          string `json:"current"`
	GuessesRemaining int    `json:"guesses_remaining"`
}

// reporter is a minimal testify assert.TestingT: it only needs Errorf, so
// this smoke test can use testify's matcher library (assert.Equal, assert.True,
// ...) without being a *testing.T - there's no `go test` runner here, just a
// standalone program hitting a live server. Every Errorf call is logged
// immediately via logger and recorded, so one failed assertion doesn't stop
// the run or hide the ones after it; main reports the total count and exits
// non-zero if any were recorded.
type reporter struct {
	logger   *zap.Logger
	failures int
}

func (r *reporter) Errorf(format string, args ...any) {
	r.failures++
	r.logger.Error(fmt.Sprintf(format, args...))
}

func main() {
	ctx := context.Background()
	logger := logging.NewLogger(ctx)
	rep := &reporter{logger: logger}
	a := assert.New(rep)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client := &client{logger: logger, a: a}

	userID := uuid.NewString()
	otherUserID := uuid.NewString()

	// --- Auth ---------------------------------------------------------

	client.do(ctx, "new-no-auth", "POST", "/new", authHeaders{}, nil, 401)
	client.do(ctx, "new-bad-token", "POST", "/new", authHeaders{token: "wrong", userID: userID}, nil, 401)
	client.do(ctx, "new-missing-user-role", "POST", "/new", authHeaders{token: stubServiceToken, userID: userID, noRole: true}, nil, 400)

	// --- Start a game and play it out -----------------------------------

	game := client.newGame(ctx, userID)
	a.Equal(len(game.Current), strings.Count(game.Current, "_"), "new game board should be all underscores, got %q", game.Current)
	a.Equal(6, game.GuessesRemaining, "new game should start with 6 guesses remaining")

	logger.Info("started game", zap.String("id", game.ID), zap.String("board", game.Current), zap.Int("guesses_remaining", game.GuessesRemaining))

	// --- Guess validation ------------------------------------------------

	client.do(ctx, "guess-two-chars", "POST", "/guess", auth(userID), mustJSON(guessBody(game.ID, "AB")), 400)
	client.do(ctx, "guess-non-letter", "POST", "/guess", auth(userID), mustJSON(guessBody(game.ID, "1")), 400)
	client.do(ctx, "guess-bad-game-id", "POST", "/guess", auth(userID), mustJSON(guessBody("not-a-uuid", "A")), 400)
	client.do(ctx, "guess-missing-game", "POST", "/guess", auth(userID), mustJSON(guessBody(uuid.NewString(), "A")), 404)
	client.do(ctx, "guess-wrong-owner", "POST", "/guess", auth(otherUserID), mustJSON(guessBody(game.ID, "A")), 403)

	// --- Play the real game out, one letter at a time, until it ends -----
	// The word is unknown and random, so this drives real play rather than
	// asserting a specific outcome: on every guess, the board can only grow
	// more revealed (never lose an already-revealed letter) and
	// guesses_remaining can only go down by exactly 1 on a miss, or stay put
	// on a hit. guesses_remaining starts at 6, so this is guaranteed to
	// finish (win or lose) well within the alphabet's 26 letters.
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	prev := game
	var completed bool
	for i := 0; i < len(alphabet) && !completed; i++ {
		letter := string(alphabet[i])
		next := client.guess(ctx, "guess-"+letter, userID, game.ID, letter)

		a.Len(next.Current, len(prev.Current), "board length changed on guess %q: %q -> %q", letter, prev.Current, next.Current)
		for pos := range prev.Current {
			if prev.Current[pos] != '_' && pos < len(next.Current) {
				a.Equal(prev.Current[pos], next.Current[pos], "an already-revealed letter changed on guess %q: %q -> %q", letter, prev.Current, next.Current)
			}
		}

		if strings.ContainsRune(next.Current, rune(letter[0])) {
			a.Equal(prev.GuessesRemaining, next.GuessesRemaining, "a hit for %q shouldn't change guesses_remaining", letter)
		} else {
			a.Equal(prev.GuessesRemaining-1, next.GuessesRemaining, "a miss for %q should decrement guesses_remaining by 1", letter)
		}

		completed = next.GuessesRemaining == 0 || !strings.Contains(next.Current, "_")
		prev = next
	}
	a.True(completed, "game never reached a win/loss state within %d letters", len(alphabet))
	logger.Info("game completed", zap.String("id", prev.ID), zap.String("board", prev.Current), zap.Int("guesses_remaining", prev.GuessesRemaining))

	// --- A completed game rejects further guesses ------------------------

	client.do(ctx, "guess-after-completed", "POST", "/guess", auth(userID), mustJSON(guessBody(game.ID, "A")), 409)

	// --- Unrouted path ----------------------------------------------------

	client.do(ctx, "unrouted", "GET", "/nope", auth(userID), nil, 404)

	if rep.failures > 0 {
		logger.Error("smoke test failed", zap.Int("failed_assertions", rep.failures))
		os.Exit(1)
	}
	logger.Info("all assertions passed")
}

type authHeaders struct {
	token  string
	userID string
	noRole bool
}

func auth(userID string) authHeaders {
	return authHeaders{token: stubServiceToken, userID: userID}
}

func guessBody(gameID, letter string) map[string]string {
	return map[string]string{"id": gameID, "guess": letter}
}

// client bundles the shared dependencies every request-issuing helper needs.
type client struct {
	logger *zap.Logger
	a      *assert.Assertions
}

func (c *client) newGame(ctx context.Context, userID string) gameState {
	body := c.do(ctx, "new", "POST", "/new", auth(userID), nil, 201)
	var g gameState
	if err := json.Unmarshal(body, &g); err != nil {
		c.logger.Fatal("unmarshal new game response", zap.Error(err))
	}
	return g
}

func (c *client) guess(ctx context.Context, id, userID, gameID, letter string) gameState {
	body := c.do(ctx, id, "POST", "/guess", auth(userID), mustJSON(guessBody(gameID, letter)), 200)
	var g gameState
	if err := json.Unmarshal(body, &g); err != nil {
		c.logger.Fatal("unmarshal guess response", zap.Error(err))
	}
	return g
}

// do issues a plain HTTP request against the running server, asserts its
// status matches wantStatus, and returns the response body.
func (c *client) do(ctx context.Context, id, method, path string, headers authHeaders, body []byte, wantStatus int) []byte {
	var raw io.Reader
	if body != nil {
		raw = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, raw)
	if err != nil {
		c.logger.Fatal("build request", zap.String("id", id), zap.Error(err))
	}
	if headers.token != "" {
		req.Header.Set("Authorization", "Bearer "+headers.token)
	}
	if headers.userID != "" {
		req.Header.Set("X-User-Id", headers.userID)
	}
	if !headers.noRole && headers.userID != "" {
		req.Header.Set("X-User-Role", "client")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.logger.Fatal("issue request", zap.String("id", id), zap.Error(err))
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logger.Fatal("read response", zap.String("id", id), zap.Error(err))
	}

	c.logger.Info("request", zap.String("id", id), zap.String("method", method), zap.String("path", path),
		zap.Int("status", resp.StatusCode), zap.ByteString("body", respBody))
	c.a.Equal(wantStatus, resp.StatusCode, "[%s] %s %s", id, method, path)
	return respBody
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
