package rest_test

import (
	"context"

	"github.com/google/uuid"

	"github.com/maxiancillotti/wordgame/internal/models"
	"github.com/maxiancillotti/wordgame/internal/service"
)

// fakeServicer is a hand-rolled service.Servicer for transport-layer tests.
type fakeServicer struct {
	newGameFn     func(ctx context.Context, userID uuid.UUID) (models.Game, error)
	guessLetterFn func(ctx context.Context, userID, gameID uuid.UUID, guess string) (models.Game, error)
}

var _ service.Servicer = (*fakeServicer)(nil)

func (f *fakeServicer) NewGame(ctx context.Context, userID uuid.UUID) (models.Game, error) {
	return f.newGameFn(ctx, userID)
}

func (f *fakeServicer) GuessLetter(ctx context.Context, userID, gameID uuid.UUID, guess string) (models.Game, error) {
	return f.guessLetterFn(ctx, userID, gameID, guess)
}
