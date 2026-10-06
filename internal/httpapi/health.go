package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

func healthHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			logger.Error("encode health response", "error", err)
		}
	}
}
