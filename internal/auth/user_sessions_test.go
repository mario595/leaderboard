package auth

import (
	"testing"

	"github.com/alexedwards/scs/v2"

	"github.com/mario595/leaderboard/internal/testutil"
)

// The real session store (sqlite3store): Iterate needs a store that can
// list every session, and this one is what production uses.
func TestDestroyUserSessions(t *testing.T) {
	sessions, stop := NewSessionManager(newTestDB(t), false)
	t.Cleanup(stop)
	phone := newUserSession(t, sessions, 42)
	laptop := newUserSession(t, sessions, 42)
	other := newUserSession(t, sessions, 7)
	anonymous := newSession(t, sessions, "something", "else")

	n, err := DestroyUserSessions(t.Context(), sessions, 42)
	if err != nil {
		t.Fatalf("DestroyUserSessions: %v", err)
	}

	testutil.AssertEqual(t, "destroyed", 2, n)
	testutil.AssertEqual(t, "phone session user", int64(0), sessionUser(t, sessions, phone))
	testutil.AssertEqual(t, "laptop session user", int64(0), sessionUser(t, sessions, laptop))
	// Positive controls: other members and anonymous sessions survive.
	testutil.AssertEqual(t, "other member's session user", int64(7), sessionUser(t, sessions, other))
	ctx, err := sessions.Load(t.Context(), anonymous)
	if err != nil {
		t.Fatalf("load anonymous session: %v", err)
	}
	testutil.AssertEqual(t, "anonymous session value", "else", sessions.GetString(ctx, "something"))
}

func TestDestroyUserSessions_unknownUser(t *testing.T) {
	sessions, stop := NewSessionManager(newTestDB(t), false)
	t.Cleanup(stop)
	tok := newUserSession(t, sessions, 7)

	n, err := DestroyUserSessions(t.Context(), sessions, 42)
	if err != nil {
		t.Fatalf("DestroyUserSessions: %v", err)
	}

	testutil.AssertEqual(t, "destroyed", 0, n)
	testutil.AssertEqual(t, "session user", int64(7), sessionUser(t, sessions, tok))
}

// newUserSession stores a logged-in session for userID and returns its token.
func newUserSession(t *testing.T, sessions *scs.SessionManager, userID int64) string {
	t.Helper()
	return newSession(t, sessions, SessionUserIDKey, userID)
}

// newSession stores a session holding key = value and returns its token,
// without an HTTP round trip: Load with no token starts a new session.
func newSession(t *testing.T, sessions *scs.SessionManager, key string, value any) string {
	t.Helper()
	ctx, err := sessions.Load(t.Context(), "")
	if err != nil {
		t.Fatalf("load new session: %v", err)
	}
	sessions.Put(ctx, key, value)
	token, _, err := sessions.Commit(ctx)
	if err != nil {
		t.Fatalf("commit session: %v", err)
	}
	return token
}

// sessionUser returns the user ID stored in the session with token, 0 when
// the session is gone (Load of an unknown token starts an empty one).
func sessionUser(t *testing.T, sessions *scs.SessionManager, token string) int64 {
	t.Helper()
	ctx, err := sessions.Load(t.Context(), token)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	return sessions.GetInt64(ctx, SessionUserIDKey)
}
