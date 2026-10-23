package httpapi

import (
	"context"
	"log/slog"
	"net/http"

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

// meHandler answers GET /api/me with the logged-in user, without tokens. It
// must sit behind requireAuth, which puts the user in the context.
func meHandler(logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			logger.Error("no user in context: route not behind requireAuth")
			writeError(w, logger, http.StatusInternalServerError, "internal error")
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
