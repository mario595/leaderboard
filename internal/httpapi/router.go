package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NewRouter builds the http router with all the application routes.
func NewRouter(logger *slog.Logger) http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", healthHandler(logger))

	return r
}
