package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			status := http.StatusOK
			if ww.Status() > 0 {
				status = ww.Status()
			}
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration", time.Since(start),
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}
