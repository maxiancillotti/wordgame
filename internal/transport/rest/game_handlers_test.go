package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/maxi.ancillotti/maxkit/pkg/apperr"
	"gitlab.com/maxi.ancillotti/maxkit/pkg/reqctx"
	"gitlab.com/maxi.ancillotti/maxkit/pkg/users"

	"github.com/maxiancillotti/wordgame/internal/models"
	"github.com/maxiancillotti/wordgame/internal/transport/rest"
)

// authedRequest builds a request carrying a reqctx.User, standing in for
// what maxkit's auth middleware attaches in production - this package tests
// the router/handlers in isolation from that middleware.
func authedRequest(method, target string, body []byte, userID uuid.UUID) *http.Request {
	var r *http.Request
	if body != nil {
		r = httptest.NewRequest(method, target, bytes.NewReader(body))
	} else {
		r = httptest.NewRequest(method, target, nil)
	}
	ctx := reqctx.WithUser(r.Context(), users.User{ID: userID.String(), Role: users.UserRoleClient})
	return r.WithContext(ctx)
}

func decodeGameState(t *testing.T, rec *httptest.ResponseRecorder) rest.GameStateDTO {
	t.Helper()
	var dto rest.GameStateDTO
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&dto))
	return dto
}

func TestHandleNewGame_Success(t *testing.T) {
	userID := uuid.New()
	game := models.Game{UUID: uuid.New(), GuessedCurrently: "_____", GuessesRemaining: 6}

	svc := &fakeServicer{
		newGameFn: func(ctx context.Context, gotUserID uuid.UUID) (models.Game, error) {
			assert.Equal(t, userID, gotUserID)
			return game, nil
		},
	}

	router := rest.NewRouter(svc)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/new", nil, userID))

	require.Equal(t, http.StatusCreated, rec.Code)
	dto := decodeGameState(t, rec)
	assert.Equal(t, game.UUID.String(), dto.ID)
	assert.Equal(t, "_____", dto.Current)
	assert.Equal(t, 6, dto.GuessesRemaining)
}

func TestHandleGuess_Success(t *testing.T) {
	userID := uuid.New()
	gameID := uuid.New()
	updated := models.Game{UUID: gameID, GuessedCurrently: "_PP__", GuessesRemaining: 6}

	svc := &fakeServicer{
		guessLetterFn: func(ctx context.Context, gotUserID, gotGameID uuid.UUID, guess string) (models.Game, error) {
			assert.Equal(t, userID, gotUserID)
			assert.Equal(t, gameID, gotGameID)
			assert.Equal(t, "P", guess)
			return updated, nil
		},
	}

	body, err := json.Marshal(rest.GuessRequest{ID: gameID.String(), Guess: "P"})
	require.NoError(t, err)

	router := rest.NewRouter(svc)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/guess", body, userID))

	require.Equal(t, http.StatusOK, rec.Code)
	dto := decodeGameState(t, rec)
	assert.Equal(t, "_PP__", dto.Current)
}

func TestHandleGuess_InvalidGameID(t *testing.T) {
	svc := &fakeServicer{}
	body, err := json.Marshal(rest.GuessRequest{ID: "not-a-uuid", Guess: "P"})
	require.NoError(t, err)

	router := rest.NewRouter(svc)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/guess", body, uuid.New()))

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleGuess_ServiceError_NotFound(t *testing.T) {
	svc := &fakeServicer{
		guessLetterFn: func(ctx context.Context, userID, gameID uuid.UUID, guess string) (models.Game, error) {
			return models.Game{}, apperr.NotFound("game not found", nil)
		},
	}

	body, err := json.Marshal(rest.GuessRequest{ID: uuid.New().String(), Guess: "P"})
	require.NoError(t, err)

	router := rest.NewRouter(svc)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/guess", body, uuid.New()))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleGuess_ServiceError_StateConflict(t *testing.T) {
	svc := &fakeServicer{
		guessLetterFn: func(ctx context.Context, userID, gameID uuid.UUID, guess string) (models.Game, error) {
			return models.Game{}, apperr.StateConflict("game already completed", nil)
		},
	}

	body, err := json.Marshal(rest.GuessRequest{ID: uuid.New().String(), Guess: "P"})
	require.NoError(t, err)

	router := rest.NewRouter(svc)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, authedRequest(http.MethodPost, "/guess", body, uuid.New()))

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestHandleNewGame_NoUserInContext(t *testing.T) {
	svc := &fakeServicer{}
	router := rest.NewRouter(svc)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/new", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUnroutedPath(t *testing.T) {
	svc := &fakeServicer{}
	router := rest.NewRouter(svc)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
