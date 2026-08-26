package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/maxiancillotti/wordgame/internal/models"
	"github.com/maxiancillotti/wordgame/internal/service"
)

// CreateGame persists a new game. game.WordID is the words.id primary key
// (an int, for a cheap FK/index), not the word's UUID.
func (s *store) CreateGame(ctx context.Context, game service.GameCreate) error {
	_, err := s.dbconn.Exec(ctx, `
		INSERT INTO games (uuid, user_id, word_id, guessed_currently, guesses_remaining, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		game.UUID, game.UserID, game.WordID, game.GuessedCurrently, game.GuessesRemaining, game.CreatedAt, game.UpdatedAt,
	)
	if err != nil {
		return translateErr(err)
	}
	return nil
}

// GetGameWithWord returns the game identified by gameUUID along with its
// chosen word, joining games -> words on the words.id FK. The returned
// models.Game.WordID is the word's UUID (per the models package's own
// field type), populated from the joined row - games.word_id itself stores
// the cheaper int FK internally, an implementation detail of this package.
func (s *store) GetGameWithWord(ctx context.Context, gameUUID uuid.UUID) (models.Game, models.Word, error) {
	var game models.Game
	var word models.Word

	err := s.dbconn.QueryRow(ctx, `
		SELECT g.uuid, g.user_id, g.guessed_currently, g.guesses_remaining, g.created_at, g.updated_at,
		       w.id, w.uuid, w.word
		FROM games g
		JOIN words w ON w.id = g.word_id
		WHERE g.uuid = $1`,
		gameUUID,
	).Scan(
		&game.UUID, &game.UserID, &game.GuessedCurrently, &game.GuessesRemaining, &game.CreatedAt, &game.UpdatedAt,
		&word.ID, &word.UUID, &word.Word,
	)
	if err != nil {
		return models.Game{}, models.Word{}, translateErr(err)
	}

	game.WordID = word.UUID
	return game, word, nil
}

// UpdateGameProgress persists the updated board state and guesses remaining
// for the game identified by gameUUID.
func (s *store) UpdateGameProgress(ctx context.Context, gameUUID uuid.UUID, current string, guessesRemaining int, updatedAt time.Time) error {
	tag, err := s.dbconn.Exec(ctx, `
		UPDATE games SET guessed_currently = $1, guesses_remaining = $2, updated_at = $3
		WHERE uuid = $4`,
		current, guessesRemaining, updatedAt, gameUUID,
	)
	if err != nil {
		return translateErr(err)
	}
	return notFoundIfNoRows(tag)
}
