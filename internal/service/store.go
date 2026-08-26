package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/maxiancillotti/wordgame/internal/models"
)

// ErrNotFound indicates the requested record doesn't exist.
var ErrNotFound = errors.New("not found")

// Storer is the persistence port the service layer depends on. It's
// implemented by a concrete store (e.g. internal/store), which the service
// layer never depends on directly.
type Storer interface {
	// GetMaxWordID returns the highest ID in the words table. words.id is a
	// contiguous BIGSERIAL sequence populated by a single seed run with no
	// deletes, so this doubles as "how many words exist" - and MAX(id) is
	// an O(log N) backward index scan on the primary key, unlike
	// COUNT(*), which is an O(N) full scan under MVCC. Called once at
	// startup and cached by the caller, since the words table is never
	// mutated at runtime.
	GetMaxWordID(ctx context.Context) (int, error)
	// GetWordByID returns the word with the given primary key ID, or
	// ErrNotFound if it doesn't exist. A direct primary-key point lookup
	// (O(log N) via the words table's PK index) - picking which ID to look
	// up is the service layer's job, since it's the randomness decision.
	GetWordByID(ctx context.Context, id int) (models.Word, error)

	// CreateGame persists a new game.
	CreateGame(ctx context.Context, game GameCreate) error
	// GetGameWithWord returns the game identified by gameUUID along with
	// its chosen word, or ErrNotFound if it doesn't exist.
	GetGameWithWord(ctx context.Context, gameUUID uuid.UUID) (models.Game, models.Word, error)
	// UpdateGameProgress persists the updated board state and guesses
	// remaining for the game identified by gameUUID.
	UpdateGameProgress(ctx context.Context, gameUUID uuid.UUID, current string, guessesRemaining int, updatedAt time.Time) error
}

// GameCreate is the store-layer payload for creating a game. UUID,
// GuessedCurrently, GuessesRemaining, CreatedAt, and UpdatedAt are generated
// in the service layer before the store is ever called.
type GameCreate struct {
	UUID   uuid.UUID
	UserID uuid.UUID
	WordID int

	GuessedCurrently string
	GuessesRemaining int

	CreatedAt time.Time
	UpdatedAt time.Time
}
