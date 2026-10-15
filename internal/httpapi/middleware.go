package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mario595/leaderboard/internal/auth"
)

// requestLogger logs one line per request: method, path, status, duration and
// request id. Only the path is logged, never the query string, because it can
// hold secrets such as OAuth codes.
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

// userIDKey is the context key under which requireAuth stores the id of the
// logged-in user. It is an unexported type, so no other package can read or
// overwrite the value by accident.
type userIDKey struct{}

// requireAuth rejects requests that have no logged-in user with a 401 and a
// JSON error. For the others it stores the user id in the request context,
// where userIDFromContext reads it. It relies on the session middleware
// (LoadAndSave) having run first.
func requireAuth(sessions *scs.SessionManager, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := sessions.GetInt64(r.Context(), auth.SessionUserIDKey)
			if userID == 0 {
				writeError(w, logger, http.StatusUnauthorized, "unauthorized")
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey{}, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// userIDFromContext returns the id of the logged-in user stored by requireAuth.
// The boolean is false when the request did not go through requireAuth.
func userIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDKey{}).(int64)
	return id, ok
}
