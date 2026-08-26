package service_test

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/maxiancillotti/wordgame/internal/models"
	"github.com/maxiancillotti/wordgame/internal/service"
)

// fakeStore is a hand-rolled in-memory service.Storer for business-layer
// tests.
type fakeStore struct {
	maxWordID int
	words     map[int]models.Word

	games map[uuid.UUID]models.Game
}

var _ service.Storer = (*fakeStore)(nil)

func newFakeStore(words ...models.Word) *fakeStore {
	fs := &fakeStore{
		words: make(map[int]models.Word),
		games: make(map[uuid.UUID]models.Game),
	}
	for _, w := range words {
		fs.words[w.ID] = w
		if w.ID > fs.maxWordID {
			fs.maxWordID = w.ID
		}
	}
	return fs
}

func (fs *fakeStore) GetMaxWordID(ctx context.Context) (int, error) {
	return fs.maxWordID, nil
}

func (fs *fakeStore) GetWordByID(ctx context.Context, id int) (models.Word, error) {
	w, ok := fs.words[id]
	if !ok {
		return models.Word{}, service.ErrNotFound
	}
	return w, nil
}

func (fs *fakeStore) CreateGame(ctx context.Context, g service.GameCreate) error {
	word := fs.words[g.WordID]
	fs.games[g.UUID] = models.Game{
		UUID:             g.UUID,
		UserID:           g.UserID,
		WordID:           word.UUID,
		GuessedCurrently: g.GuessedCurrently,
		GuessesRemaining: g.GuessesRemaining,
		CreatedAt:        g.CreatedAt,
		UpdatedAt:        g.UpdatedAt,
	}
	return nil
}

func (fs *fakeStore) GetGameWithWord(ctx context.Context, gameUUID uuid.UUID) (models.Game, models.Word, error) {
	g, ok := fs.games[gameUUID]
	if !ok {
		return models.Game{}, models.Word{}, service.ErrNotFound
	}
	for _, w := range fs.words {
		if w.UUID == g.WordID {
			return g, w, nil
		}
	}
	return models.Game{}, models.Word{}, service.ErrNotFound
}

func (fs *fakeStore) UpdateGameProgress(ctx context.Context, gameUUID uuid.UUID, current string, guessesRemaining int, updatedAt time.Time) error {
	g, ok := fs.games[gameUUID]
	if !ok {
		return service.ErrNotFound
	}
	g.GuessedCurrently = current
	g.GuessesRemaining = guessesRemaining
	g.UpdatedAt = updatedAt
	fs.games[gameUUID] = g
	return nil
}
