package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/testutil"
)

// newLogoutApp builds a small app around the real Logout handler:
//
//	/login   creates a session holding a value
//	/read    writes back that value
//	/logout  is the Logout handler
//
// The router restricts logout to POST, but the handler itself does not care
// about the method, and the do helper sends GET requests.
func newLogoutApp(t *testing.T, h *Handler) http.Handler {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		h.sessions.Put(r.Context(), "k", "v")
	})
	mux.HandleFunc("GET /read", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, h.sessions.GetString(r.Context(), "k"))
	})
	mux.HandleFunc("GET /logout", h.Logout)
	return h.sessions.LoadAndSave(mux)
}

func TestLogout_endsTheSession(t *testing.T) {
	app := newLogoutApp(t, newTestHandler(false))
	cookie := sessionCookie(t, do(t, app, "/login"))

	r := do(t, app, "/logout", cookie)

	testutil.AssertEqual(t, "status", http.StatusNoContent, r.status)

	// The response expires the session cookie in the browser.
	expired := sessionCookie(t, r)
	testutil.AssertEqual(t, "cookie value", "", expired.Value)
	if expired.MaxAge >= 0 {
		t.Errorf("cookie MaxAge = %d, want a negative value (the cookie is deleted)", expired.MaxAge)
	}
}

func TestLogout_oldTokenStopsWorking(t *testing.T) {
	app := newLogoutApp(t, newTestHandler(false))
	cookie := sessionCookie(t, do(t, app, "/login"))

	// Before the logout, the token gives access to the session.
	testutil.AssertEqual(t, "value before logout", "v", do(t, app, "/read", cookie).body)

	do(t, app, "/logout", cookie)

	// A browser that kept the old cookie gets nothing: the session is gone from the store.
	testutil.AssertEqual(t, "value after logout", "", do(t, app, "/read", cookie).body)
}

func TestLogout_withoutSession(t *testing.T) {
	app := newLogoutApp(t, newTestHandler(false))

	r := do(t, app, "/logout")

	testutil.AssertEqual(t, "status", http.StatusNoContent, r.status)
}

// failingDeleteStore is a session store that works, except that it cannot delete.
type failingDeleteStore struct {
	data map[string][]byte
}

func (s *failingDeleteStore) Find(token string) ([]byte, bool, error) {
	b, ok := s.data[token]
	return b, ok, nil
}

func (s *failingDeleteStore) Commit(token string, b []byte, _ time.Time) error {
	s.data[token] = b
	return nil
}

func (s *failingDeleteStore) Delete(string) error {
	return errors.New("store unavailable")
}

// TestLogout_storeFailure checks that a logout that could not remove the session
// reports it, instead of claiming success, and does not leak the internal error.
func TestLogout_storeFailure(t *testing.T) {
	h := newTestHandler(false)
	h.sessions.Store = &failingDeleteStore{data: map[string][]byte{}}
	app := newLogoutApp(t, h)
	cookie := sessionCookie(t, do(t, app, "/login"))

	r := do(t, app, "/logout", cookie)

	testutil.AssertEqual(t, "status", http.StatusInternalServerError, r.status)
	if strings.Contains(r.body, "store unavailable") {
		t.Errorf("body %q leaks the internal error", r.body)
	}
}
