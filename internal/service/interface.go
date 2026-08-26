package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/maxiancillotti/wordgame/internal/models"
)

// Servicer is the business layer's public API: it validates input, enforces
// the game's rules, and delegates persistence to a Storer.
type Servicer interface {
	// NewGame picks a random word and starts a new game for userID.
	NewGame(ctx context.Context, userID uuid.UUID) (models.Game, error)
	// GuessLetter evaluates guess against the game identified by gameID and
	// persists the updated progress. Returns apperr.NotFound if gameID
	// doesn't exist, apperr.Forbidden if the game doesn't belong to userID,
	// or apperr.StateConflict if the game is already completed.
	GuessLetter(ctx context.Context, userID, gameID uuid.UUID, guess string) (models.Game, error)
}
