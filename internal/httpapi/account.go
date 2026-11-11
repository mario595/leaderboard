package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/mario595/leaderboard/internal/auth"
	"github.com/mario595/leaderboard/internal/store"
)

// backupRetentionDays is how long a deleted member's data can survive in
// backups: the daily volume snapshots are kept 14 days (fly.toml,
// snapshot_retention). Shown on the deletion confirmation.
const backupRetentionDays = 14

// revokeFunc revokes the app's access to the user on Strava. main wires it to
// strava.Client.Deauthorize.
type revokeFunc func(ctx context.Context, u store.User) error

// userDeleter deletes everything stored about an athlete;
// *store.UserStore satisfies it.
type userDeleter interface {
	DeleteByStravaID(ctx context.Context, stravaID int64) (bool, error)
}

// deleteAccountResponse is the written confirmation of a deletion (Strava
// API Policy 2.5): when it happened (RFC 3339, UTC) and the date by which it
// is gone from backups too.
type deleteAccountResponse struct {
	DeletedAt     string `json:"deletedAt"`
	BackupsGoneBy string `json:"backupsGoneBy"`
}

// deleteAccountHandler serves DELETE /api/me: the logged-in member leaves
// and everything about them is deleted. It revokes the app's access on
// Strava, deletes the member (activities, webhook events and backfill jobs
// with them, the same path as a deauthorization), destroys every session of
// the member and answers 200 with the confirmation. The member always comes
// from the session (requireAuth), never from a parameter.
//
// A failed revoke is logged at Warn and the deletion goes on: the member
// asked to leave, and Strava's own revoke page still works. A failed delete
// answers 500 and keeps the session. A failed session cleanup after the
// delete is logged and still answers 200: requireAuth ends any leftover
// session, since the user is gone. Once started, the work is not cut short
// by the browser going away.
func deleteAccountHandler(revoke revokeFunc, users userDeleter, sessions *scs.SessionManager, now func() time.Time, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			logger.Error("no user in context: route not behind requireAuth")
			writeError(w, logger, http.StatusInternalServerError, "internal error")
			return
		}
		ctx := context.WithoutCancel(r.Context())

		if err := revoke(ctx, u); err != nil {
			logger.Warn("revoke strava access, deleting anyway", "user_id", u.ID, "error", err)
		}

		if _, err := users.DeleteByStravaID(ctx, u.StravaID); err != nil {
			logger.Error("delete account", "user_id", u.ID, "error", err)
			writeError(w, logger, http.StatusInternalServerError, "internal error")
			return
		}

		// This request's session first, so the response expires the cookie;
		// then the member's sessions on other devices.
		if err := sessions.Destroy(r.Context()); err != nil {
			logger.Error("destroy session after account deletion", "user_id", u.ID, "error", err)
		}
		others, err := auth.DestroyUserSessions(ctx, sessions, u.ID)
		if err != nil {
			logger.Error("destroy other sessions after account deletion", "user_id", u.ID, "error", err)
		}

		deletedAt := now().UTC()
		logger.Info("account deleted", "user_id", u.ID, "other_sessions", others)
		writeJSON(w, logger, http.StatusOK, deleteAccountResponse{
			DeletedAt:     deletedAt.Format(time.RFC3339),
			BackupsGoneBy: deletedAt.AddDate(0, 0, backupRetentionDays).Format(time.DateOnly),
		})
	}
}
