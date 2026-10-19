package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/mario595/leaderboard/internal/store"
)

type userGetter interface {
	GetByID(ctx context.Context, id int64) (store.User, error)
}

type meResponse struct {
	ID        int64  `json:"id"`
	StravaID  int64  `json:"stravaId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func meHandler(users userGetter, sessions *scs.SessionManager, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := userIDFromContext(r.Context())
		if !ok {
			logger.Error("userID not found in Context")
			writeError(w, logger, http.StatusUnauthorized, "unauthorized user")
			return
		}
		u, err := users.GetByID(r.Context(), id)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				logger.Info("user not found. Destroying session", "userID", id)
				_ = sessions.Destroy(r.Context())
				writeError(w, logger, http.StatusUnauthorized, "unauthorized user")
				return
			}
			logger.Error("error in getByID", "error", err, "userID", id)
			writeError(w, logger, http.StatusInternalServerError, "error in /me")
			return
		}
		writeJSON(
			w,
			logger,
			http.StatusOK,
			meResponse{
				ID:        u.ID,
				StravaID:  u.StravaID,
				FirstName: u.FirstName,
				LastName:  u.LastName,
			},
		)
	}
}
