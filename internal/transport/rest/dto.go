package rest

import "github.com/maxiancillotti/wordgame/internal/models"

// GameStateDTO is the response shape for both POST /new and POST /guess,
// per the README's API spec.
type GameStateDTO struct {
	ID               string `json:"id"`
	Current          string `json:"current"`
	GuessesRemaining int    `json:"guesses_remaining"`
}

// NewGameStateDTO maps a models.Game to its wire representation.
func NewGameStateDTO(g models.Game) GameStateDTO {
	return GameStateDTO{
		ID:               g.UUID.String(),
		Current:          g.GuessedCurrently,
		GuessesRemaining: g.GuessesRemaining,
	}
}

// GuessRequest is the request body for POST /guess.
type GuessRequest struct {
	ID    string `json:"id"`
	Guess string `json:"guess"`
}
