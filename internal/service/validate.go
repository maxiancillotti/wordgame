package service

import "gitlab.com/maxi.ancillotti/maxkit/pkg/apperr"

// validateGuess ensures guess is exactly one ASCII letter.
func validateGuess(guess string) error {
	if len(guess) != 1 {
		return apperr.BadUserInput("guess must be exactly one character", nil)
	}
	c := guess[0]
	if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
		return apperr.BadUserInput("guess must be a letter A-Z", nil)
	}
	return nil
}
