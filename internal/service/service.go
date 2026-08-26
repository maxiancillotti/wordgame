// Package service implements the word-guessing game's business rules on top
// of a Storer persistence port.
package service

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"

	"gitlab.com/maxi.ancillotti/maxkit/pkg/apperr"

	"github.com/maxiancillotti/wordgame/internal/models"
)

// initialGuesses is how many wrong guesses a new game allows, per the
// README's spec.
const initialGuesses = 6

// service implements Servicer on top of a Storer. wordCount is fetched once
// at construction and cached for the life of the process: the words table
// is seeded once and never mutated at runtime, so a fresh COUNT(*) on every
// new game would be a wasted full-table scan.
type service struct {
	store     Storer
	maxWordID int
}

var _ Servicer = (*service)(nil)

// NewService returns a Servicer backed by store. It queries store once for
// the current max word ID; a failure here fails fast rather than surfacing
// as a broken NewGame call later.
func NewService(store Storer) (*service, error) {
	if store == nil {
		return nil, errors.New("service: new service: store cannot be nil")
	}

	maxID, err := store.GetMaxWordID(context.Background())
	if err != nil {
		return nil, errors.New("service: new service: get max word id: " + err.Error())
	}
	if maxID == 0 {
		return nil, errors.New("service: new service: words table is empty")
	}

	return &service{store: store, maxWordID: maxID}, nil
}

func (s *service) NewGame(ctx context.Context, userID uuid.UUID) (models.Game, error) {
	// words.id is a contiguous BIGSERIAL sequence starting at 1, populated
	// by a single seed run with no deletes - so any int in [1, wordCount]
	// is a valid ID, letting the store do a direct PK point lookup instead
	// of an OFFSET scan.
	wordID := rand.Intn(s.maxWordID) + 1 //nolint:gosec // picking a game word, not a security-sensitive value - crypto/rand buys nothing here
	word, err := s.store.GetWordByID(ctx, wordID)
	if err != nil {
		return models.Game{}, apperr.NotFound("could not pick a word", err)
	}

	now := time.Now().UTC()
	create := GameCreate{
		UUID:             uuid.New(),
		UserID:           userID,
		WordID:           word.ID,
		GuessedCurrently: strings.Repeat("_", len(word.Word)),
		GuessesRemaining: initialGuesses,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.store.CreateGame(ctx, create); err != nil {
		return models.Game{}, err
	}

	return models.Game{
		UUID:             create.UUID,
		UserID:           create.UserID,
		WordID:           word.UUID,
		GuessedCurrently: create.GuessedCurrently,
		GuessesRemaining: create.GuessesRemaining,
		CreatedAt:        create.CreatedAt,
		UpdatedAt:        create.UpdatedAt,
	}, nil
}

func (s *service) GuessLetter(ctx context.Context, userID, gameID uuid.UUID, guess string) (models.Game, error) {
	if err := validateGuess(guess); err != nil {
		return models.Game{}, err
	}
	guess = strings.ToUpper(guess)

	game, word, err := s.store.GetGameWithWord(ctx, gameID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return models.Game{}, apperr.NotFound("game not found", err)
		}
		return models.Game{}, err
	}

	if game.UserID != userID {
		return models.Game{}, apperr.Forbidden("game does not belong to this user", nil)
	}

	if isCompleted(game.GuessedCurrently, game.GuessesRemaining) {
		return models.Game{}, apperr.StateConflict("game already completed", nil)
	}

	if strings.Contains(word.Word, guess) {
		game.GuessedCurrently = reveal(game.GuessedCurrently, word.Word, guess)
	} else {
		game.GuessesRemaining--
	}
	game.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateGameProgress(ctx, gameID, game.GuessedCurrently, game.GuessesRemaining, game.UpdatedAt); err != nil {
		return models.Game{}, err
	}

	return game, nil
}

// reveal returns a copy of current with every position where word matches
// guess replaced by guess, leaving every other position untouched.
func reveal(current, word, guess string) string {
	b := []byte(current)
	for i := 0; i < len(word); i++ {
		if string(word[i]) == guess {
			b[i] = guess[0]
		}
	}
	return string(b)
}

// isCompleted reports whether a game has already ended, either by running
// out of guesses or by fully revealing the word.
func isCompleted(current string, guessesRemaining int) bool {
	return guessesRemaining <= 0 || !strings.Contains(current, "_")
}
