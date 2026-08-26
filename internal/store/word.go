package store

import (
	"context"

	"github.com/maxiancillotti/wordgame/internal/models"
)

// GetMaxWordID returns the highest ID in the words table. Postgres rewrites
// MAX() on an indexed column into a backward index-only scan with LIMIT 1
// (the same plan ORDER BY id DESC LIMIT 1 would produce), so this is
// O(log N) rather than a full-table COUNT(*) scan - coalesce additionally
// avoids a separate empty-table/no-rows case, since an aggregate with no
// GROUP BY always returns exactly one row.
func (s *store) GetMaxWordID(ctx context.Context) (int, error) {
	var maxID int
	err := s.dbconn.QueryRow(ctx, `SELECT coalesce(max(id), 0) FROM words`).Scan(&maxID)
	if err != nil {
		return 0, translateErr(err)
	}
	return maxID, nil
}

// GetWordByID returns the word with the given primary key ID, or
// service.ErrNotFound if it doesn't exist. A direct primary-key point
// lookup, served by the words table's PK index.
func (s *store) GetWordByID(ctx context.Context, id int) (models.Word, error) {
	var word models.Word
	err := s.dbconn.QueryRow(ctx, `SELECT id, uuid, word FROM words WHERE id = $1`, id).
		Scan(&word.ID, &word.UUID, &word.Word)
	if err != nil {
		return models.Word{}, translateErr(err)
	}
	return word, nil
}
