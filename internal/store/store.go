// Package store is a Postgres-backed implementation of service.Storer, built
// on pgx/v5. Its assumed schema (words, games) is defined in
// migrations/000001_init_schema.up.sql - that file is the source of truth;
// this package doesn't repeat it.
package store

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maxiancillotti/wordgame/internal/service"
)

// store is a Postgres-backed service.Storer.
type store struct {
	dbconn *pgxpool.Pool
}

var _ service.Storer = (*store)(nil)

// New returns a store that runs its queries against dbconn.
func New(dbconn *pgxpool.Pool) (*store, error) {
	if dbconn == nil {
		return nil, errors.New("dbconn cannot be nil")
	}

	return &store{dbconn: dbconn}, nil
}
