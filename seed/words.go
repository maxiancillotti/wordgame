// Package seed bulk-loads words.txt into the words table at application
// startup, after migrations have run. It's safe to call on every boot,
// including from more than one concurrently starting replica: each insert
// is guarded by the words.word UNIQUE constraint via ON CONFLICT DO
// NOTHING, so a racing insert of an already-seeded word is simply skipped,
// never an error.
package seed

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"gitlab.com/maxi.ancillotti/maxkit/pkg/obs/logging"
)

// wordsRegexp matches a normalized, letters-only word.
var wordsRegexp = regexp.MustCompile("^[A-Z]+$")

// Words reads path (e.g. "words.txt") and bulk-loads its entries into the
// words table.
func Words(ctx context.Context, pool *pgxpool.Pool, path string) error {
	words, err := loadWords(path)
	if err != nil {
		return err
	}
	return seedWords(ctx, pool, words)
}

// loadWords loads and normalizes the word dictionary from path.
func loadWords(path string) ([]string, error) {
	f, err := os.Open(path) //nolint:gosec // path is always the "words.txt" constant main.go passes in, never user input
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	words := []string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		word := strings.ToUpper(strings.TrimSpace(scanner.Text()))
		if wordsRegexp.MatchString(word) {
			words = append(words, word)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return words, nil
}

// seedWords inserts words into the words table, skipping any that already
// exist.
func seedWords(ctx context.Context, pool *pgxpool.Pool, words []string) error {
	batch := &pgx.Batch{}
	for _, w := range words {
		batch.Queue(`INSERT INTO words (uuid, word) VALUES ($1, $2) ON CONFLICT (word) DO NOTHING`, uuid.New(), w)
	}

	br := pool.SendBatch(ctx, batch)
	defer func() { _ = br.Close() }()

	for range words {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}

	logging.From(ctx).Info("seed: processed words", zap.Int("count", len(words)))
	return nil
}
