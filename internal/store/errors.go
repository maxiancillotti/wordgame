package store

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/maxiancillotti/wordgame/internal/service"
)

// translateErr maps errors coming out of pgx to the sentinels
// service.Storer implementations are expected to return, wrapping anything
// else so the call site is identifiable.
func translateErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return service.ErrNotFound
	}
	return fmt.Errorf("store: %w", err)
}

// notFoundIfNoRows reports service.ErrNotFound if tag reflects zero affected
// rows, for UPDATE/DELETE statements that don't use RETURNING to detect a
// missing row.
func notFoundIfNoRows(tag pgconn.CommandTag) error {
	if tag.RowsAffected() == 0 {
		return service.ErrNotFound
	}
	return nil
}
