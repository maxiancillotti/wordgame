//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/maxiancillotti/wordgame/internal/service"
)

func validGameCreate(userID uuid.UUID, wordID int) service.GameCreate {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return service.GameCreate{
		UUID:             uuid.New(),
		UserID:           userID,
		WordID:           wordID,
		GuessedCurrently: "_____",
		GuessesRemaining: 6,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

func TestStoreCreateAndGetGameWithWord(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	word := insertWord(t, "APPLE")
	userID := uuid.New()
	create := validGameCreate(userID, word.ID)

	require.NoError(t, st.CreateGame(ctx, create))

	game, gotWord, err := st.GetGameWithWord(ctx, create.UUID)
	require.NoError(t, err)

	assert.Equal(t, create.UUID, game.UUID)
	assert.Equal(t, userID, game.UserID)
	assert.Equal(t, word.UUID, game.WordID)
	assert.Equal(t, create.GuessedCurrently, game.GuessedCurrently)
	assert.Equal(t, create.GuessesRemaining, game.GuessesRemaining)
	assert.Equal(t, word, gotWord)
}

func TestStoreGetGameWithWord_NotFound(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	_, _, err := st.GetGameWithWord(ctx, uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrNotFound))
}

func TestStoreUpdateGameProgress(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	word := insertWord(t, "APPLE")
	create := validGameCreate(uuid.New(), word.ID)
	require.NoError(t, st.CreateGame(ctx, create))

	updatedAt := time.Now().UTC().Truncate(time.Microsecond)
	require.NoError(t, st.UpdateGameProgress(ctx, create.UUID, "_PP__", 5, updatedAt))

	game, _, err := st.GetGameWithWord(ctx, create.UUID)
	require.NoError(t, err)
	assert.Equal(t, "_PP__", game.GuessedCurrently)
	assert.Equal(t, 5, game.GuessesRemaining)
	assert.WithinDuration(t, updatedAt, game.UpdatedAt, time.Second)
}

func TestStoreUpdateGameProgress_NotFound(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	err := st.UpdateGameProgress(ctx, uuid.New(), "_____", 6, time.Now())
	require.Error(t, err)
	assert.True(t, errors.Is(err, service.ErrNotFound))
}
