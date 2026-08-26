package models

import "github.com/google/uuid"

type Word struct {
	ID   int
	UUID uuid.UUID

	Word string
}
