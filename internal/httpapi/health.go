package httpapi

import (
	"log/slog"
	"net/http"
)

func healthHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, logger, http.StatusOK, map[string]string{"status": "ok"})
	}
}
