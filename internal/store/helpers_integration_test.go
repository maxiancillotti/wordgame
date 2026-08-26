//go:build integration

package store_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/maxiancillotti/wordgame/internal/models"
	"github.com/maxiancillotti/wordgame/internal/service"
	"github.com/maxiancillotti/wordgame/internal/store"
)

// truncateAll clears every table testPool's database has, so each test
// starts from an empty database regardless of what earlier tests left
// behind. Tests in this package don't run in parallel, so truncating up
// front (rather than via t.Cleanup) is enough.
func truncateAll(t *testing.T) {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `TRUNCATE games, words RESTART IDENTITY CASCADE`)
	require.NoError(t, err, "truncate tables")
}

// newTestStore truncates the database and returns a fresh service.Storer
// backed by testPool, ready for a single test to use.
func newTestStore(t *testing.T) service.Storer {
	t.Helper()
	truncateAll(t)
	st, err := store.New(testPool)
	require.NoError(t, err)
	return st
}

// insertWord inserts a single word directly (bypassing the seed.Words step,
// which this package doesn't depend on) and returns its models.Word.
func insertWord(t *testing.T, word string) models.Word {
	t.Helper()
	w := models.Word{UUID: uuid.New(), Word: word}
	err := testPool.QueryRow(context.Background(),
		`INSERT INTO words (uuid, word) VALUES ($1, $2) RETURNING id`, w.UUID, w.Word,
	).Scan(&w.ID)
	require.NoError(t, err)
	return w
}
