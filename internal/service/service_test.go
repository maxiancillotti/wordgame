package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/maxi.ancillotti/maxkit/pkg/apperr"

	"github.com/maxiancillotti/wordgame/internal/models"
	"github.com/maxiancillotti/wordgame/internal/service"
)

func newTestService(t *testing.T, words ...models.Word) (service.Servicer, *fakeStore) {
	t.Helper()
	fs := newFakeStore(words...)
	svc, err := service.NewService(fs)
	require.NoError(t, err)
	return svc, fs
}

func appleWord() models.Word {
	return models.Word{ID: 1, UUID: uuid.New(), Word: "APPLE"}
}

func TestNewGame(t *testing.T) {
	svc, _ := newTestService(t, appleWord())

	game, err := svc.NewGame(context.Background(), uuid.New())
	require.NoError(t, err)

	assert.Equal(t, "_____", game.GuessedCurrently)
	assert.Equal(t, 6, game.GuessesRemaining)
	assert.NotEqual(t, uuid.UUID{}, game.UUID)
}

func TestGuessLetter_CorrectGuess_RevealsWithoutDecrementing(t *testing.T) {
	userID := uuid.New()
	svc, _ := newTestService(t, appleWord())

	game, err := svc.NewGame(context.Background(), userID)
	require.NoError(t, err)

	updated, err := svc.GuessLetter(context.Background(), userID, game.UUID, "P")
	require.NoError(t, err)

	assert.Equal(t, "_PP__", updated.GuessedCurrently)
	assert.Equal(t, 6, updated.GuessesRemaining)
}

func TestGuessLetter_WrongGuess_Decrements(t *testing.T) {
	userID := uuid.New()
	svc, _ := newTestService(t, appleWord())

	game, err := svc.NewGame(context.Background(), userID)
	require.NoError(t, err)

	updated, err := svc.GuessLetter(context.Background(), userID, game.UUID, "Z")
	require.NoError(t, err)

	assert.Equal(t, "_____", updated.GuessedCurrently)
	assert.Equal(t, 5, updated.GuessesRemaining)
}

func TestGuessLetter_RepeatedWrongGuess_DecrementsAgain(t *testing.T) {
	userID := uuid.New()
	svc, _ := newTestService(t, appleWord())

	game, err := svc.NewGame(context.Background(), userID)
	require.NoError(t, err)

	_, err = svc.GuessLetter(context.Background(), userID, game.UUID, "Z")
	require.NoError(t, err)
	updated, err := svc.GuessLetter(context.Background(), userID, game.UUID, "Z")
	require.NoError(t, err)

	assert.Equal(t, 4, updated.GuessesRemaining)
}

func TestGuessLetter_Win(t *testing.T) {
	userID := uuid.New()
	svc, _ := newTestService(t, models.Word{ID: 1, UUID: uuid.New(), Word: "GO"})

	game, err := svc.NewGame(context.Background(), userID)
	require.NoError(t, err)

	_, err = svc.GuessLetter(context.Background(), userID, game.UUID, "G")
	require.NoError(t, err)
	updated, err := svc.GuessLetter(context.Background(), userID, game.UUID, "O")
	require.NoError(t, err)

	assert.Equal(t, "GO", updated.GuessedCurrently)
}

func TestGuessLetter_Loss(t *testing.T) {
	userID := uuid.New()
	svc, _ := newTestService(t, appleWord())

	game, err := svc.NewGame(context.Background(), userID)
	require.NoError(t, err)

	guesses := []string{"Z", "X", "Q", "J", "K"}
	var updated models.Game
	for _, g := range guesses {
		updated, err = svc.GuessLetter(context.Background(), userID, game.UUID, g)
		require.NoError(t, err)
	}

	assert.Equal(t, 1, updated.GuessesRemaining)

	updated, err = svc.GuessLetter(context.Background(), userID, game.UUID, "W")
	require.NoError(t, err)
	assert.Equal(t, 0, updated.GuessesRemaining)
}

func TestGuessLetter_OnCompletedGame_ReturnsStateConflict(t *testing.T) {
	userID := uuid.New()
	svc, _ := newTestService(t, models.Word{ID: 1, UUID: uuid.New(), Word: "GO"})

	game, err := svc.NewGame(context.Background(), userID)
	require.NoError(t, err)
	_, err = svc.GuessLetter(context.Background(), userID, game.UUID, "G")
	require.NoError(t, err)
	_, err = svc.GuessLetter(context.Background(), userID, game.UUID, "O")
	require.NoError(t, err)

	_, err = svc.GuessLetter(context.Background(), userID, game.UUID, "A")
	require.Error(t, err)
	var appErr *apperr.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperr.CodeStateConflict, appErr.Code)
}

func TestGuessLetter_InvalidGuess(t *testing.T) {
	userID := uuid.New()
	svc, _ := newTestService(t, appleWord())

	game, err := svc.NewGame(context.Background(), userID)
	require.NoError(t, err)

	_, err = svc.GuessLetter(context.Background(), userID, game.UUID, "AB")
	require.Error(t, err)
	var appErr *apperr.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperr.CodeBadUserInput, appErr.Code)
}

func TestGuessLetter_GameNotFound(t *testing.T) {
	svc, _ := newTestService(t, appleWord())

	_, err := svc.GuessLetter(context.Background(), uuid.New(), uuid.New(), "A")
	require.Error(t, err)
	var appErr *apperr.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestGuessLetter_WrongUser_ReturnsForbidden(t *testing.T) {
	svc, _ := newTestService(t, appleWord())

	game, err := svc.NewGame(context.Background(), uuid.New())
	require.NoError(t, err)

	_, err = svc.GuessLetter(context.Background(), uuid.New(), game.UUID, "A")
	require.Error(t, err)
	var appErr *apperr.AppError
	require.True(t, errors.As(err, &appErr))
	assert.Equal(t, apperr.CodeForbidden, appErr.Code)
}
