package auth

import (
	"context"
	"fmt"

	"github.com/alexedwards/scs/v2"
)

// DestroyUserSessions destroys every session of the user, on every device,
// and returns how many it destroyed. Sessions are stored as encoded blobs, so
// it walks all of them (scs Iterate; the store must list its sessions, as
// sqlite3store does). Fine for a small group; used when a member deletes
// their account.
func DestroyUserSessions(ctx context.Context, sessions *scs.SessionManager, userID int64) (int, error) {
	n := 0
	err := sessions.Iterate(ctx, func(ctx context.Context) error {
		if sessions.GetInt64(ctx, SessionUserIDKey) != userID {
			return nil
		}
		if err := sessions.Destroy(ctx); err != nil {
			return err
		}
		n++
		return nil
	})
	if err != nil {
		return n, fmt.Errorf("destroy sessions of user %d: %w", userID, err)
	}
	return n, nil
}
