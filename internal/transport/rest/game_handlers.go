package rest

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"gitlab.com/maxi.ancillotti/maxkit/pkg/apperr"
	"gitlab.com/maxi.ancillotti/maxkit/pkg/reqctx"

	"github.com/maxiancillotti/wordgame/internal/service"
)

// handleNewGame serves POST /new. The request body is ignored per spec; the
// player is identified by X-User-Id, already resolved into the request
// context by maxkit's auth middleware.
func handleNewGame(svc service.Servicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := currentUserID(r)
		if err != nil {
			apperr.WriteHTTPError(w, r.Context(), err)
			return
		}

		game, err := svc.NewGame(r.Context(), userID)
		if err != nil {
			apperr.WriteHTTPError(w, r.Context(), err)
			return
		}

		writeJSON(w, http.StatusCreated, NewGameStateDTO(game))
	}
}

// handleGuess serves POST /guess.
func handleGuess(svc service.Servicer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := currentUserID(r)
		if err != nil {
			apperr.WriteHTTPError(w, r.Context(), err)
			return
		}

		var req GuessRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apperr.WriteHTTPError(w, r.Context(), apperr.BadUserInput("invalid request body", err))
			return
		}

		gameID, err := uuid.Parse(req.ID)
		if err != nil {
			apperr.WriteHTTPError(w, r.Context(), apperr.BadUserInput("invalid game id", err))
			return
		}

		game, err := svc.GuessLetter(r.Context(), userID, gameID, req.Guess)
		if err != nil {
			apperr.WriteHTTPError(w, r.Context(), err)
			return
		}

		writeJSON(w, http.StatusOK, NewGameStateDTO(game))
	}
}

// currentUserID parses the X-User-Id-derived reqctx.User attached by
// maxkit's auth middleware as a UUID - this service's UserID type.
func currentUserID(r *http.Request) (uuid.UUID, error) {
	user, ok := reqctx.UserFromContext(r.Context())
	if !ok {
		return uuid.UUID{}, apperr.Unauthenticated("", nil)
	}
	id, err := uuid.Parse(user.ID)
	if err != nil {
		return uuid.UUID{}, apperr.BadUserInput("X-User-Id must be a valid UUID", err)
	}
	return id, nil
}
