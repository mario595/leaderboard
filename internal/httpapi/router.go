package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mario595/leaderboard/internal/auth"
)

// NewRouter builds the http router with all the application routes.
func NewRouter(logger *slog.Logger, authHandler *auth.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogger(logger))
	r.Use(middleware.Recoverer)

	r.Get("/healthz", healthHandler(logger))
	if authHandler != nil {
		r.Get("/auth/strava", authHandler.Login)
		r.Get("/auth/strava/callback", authHandler.Callback)
	}

	return r
}
