package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mario595/leaderboard/internal/auth"
	"github.com/mario595/leaderboard/internal/store"
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

// userKey is the context key under which requireAuth stores the
// logged-in user. It is an unexported type, so no other package can read or
// overwrite the value by accident.
type userKey struct{}

// requireAuth lets a request through only if its session belongs to an
// existing user, and stores that user in the request context, where
// userFromContext reads it. Without a session it answers 401. When the
// session's user no longer exists (deleted after a Strava deauthorization) it
// destroys the session and answers 401, on every route it guards. When the
// user store fails it answers 500 and keeps the session: an outage is not a
// logout. It relies on the session middleware (LoadAndSave) having run first.
func requireAuth(sessions *scs.SessionManager, users userGetter, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := sessions.GetInt64(r.Context(), auth.SessionUserIDKey)
			if userID == 0 {
				writeError(w, logger, http.StatusUnauthorized, "unauthorized")
				return
			}
			user, err := users.GetByID(r.Context(), userID)
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					logger.Info("user not found, destroying session", "user_id", userID)
					if err := sessions.Destroy(r.Context()); err != nil {
						logger.Error("could not destroy session", "user_id", userID, "error", err)
						writeError(w, logger, http.StatusUnauthorized, "unauthorized")
						return
					}
					writeError(w, logger, http.StatusUnauthorized, "unauthorized")
					return
				}
				logger.Error("look up session user", "user_id", userID, "error", err)
				writeError(w, logger, http.StatusInternalServerError, "internal error")
				return
			}
			ctx := context.WithValue(r.Context(), userKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// userFromContext returns the logged-in user stored by requireAuth.
// The boolean is false when the request did not go through requireAuth.
func userFromContext(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(userKey{}).(store.User)
	return u, ok
}
