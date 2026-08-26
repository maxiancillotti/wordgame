// Package rest is a plain net/http transport for service.Servicer: an
// ordinary *http.ServeMux dispatching to http.HandlerFunc handlers, mounted
// directly by cmd/wordgame's main.go.
package rest

import (
	"net/http"

	"gitlab.com/maxi.ancillotti/maxkit/pkg/apperr"

	"github.com/maxiancillotti/wordgame/internal/service"
)

// NewRouter builds the *http.ServeMux that dispatches this transport's
// routes onto svc.
func NewRouter(svc service.Servicer) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /new", handleNewGame(svc))
	mux.HandleFunc("POST /guess", handleGuess(svc))

	// A subtree pattern of "/" matches any request no more specific pattern
	// above matched, so unroutable requests get the same JSON error shape
	// as every other failure instead of ServeMux's default plaintext 404.
	mux.HandleFunc("/", handleUnrouted)

	return mux
}

func handleUnrouted(w http.ResponseWriter, r *http.Request) {
	apperr.WriteHTTPError(w, r.Context(), apperr.NotFound("no such endpoint: "+r.Method+" "+r.URL.Path, nil))
}
