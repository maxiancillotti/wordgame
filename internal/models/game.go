package models

import (
	"time"

	"github.com/google/uuid"
)

type Game struct {
	ID   int
	UUID uuid.UUID

	UserID uuid.UUID
	WordID uuid.UUID

	GuessedCurrently string
	GuessesRemaining int

	CreatedAt time.Time
	UpdatedAt time.Time
}
