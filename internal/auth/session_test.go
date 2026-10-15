package auth

import (
	"database/sql"
	"fmt"
	"net/http"
	"testing"
)

func TestSessionManager(t *testing.T) {

	h := newSessionApp(t, newTestDB(t), false)
	r := do(t, h, "/put")

	if r.status != http.StatusOK {
		t.Fatalf("Unexepected status: got %d, wanted %d", r.status, http.StatusOK)
	}
	if len(r.cookies) != 1 {
		t.Fatalf("Unexpected number of cookies: got %d, wanted 1", len(r.cookies))
	}
	cookie := sessionCookie(t, r)

	if cookie == nil {
		t.Fatal("no cookie named session in the response")
	}

	assertEqual(t, "cookie.HttpOnly", true, cookie.HttpOnly)
	assertEqual(t, "cookie.SameSite", http.SameSiteLaxMode, cookie.SameSite)
	assertEqual(t, "cookie.Secure", false, cookie.Secure)
	assertEqual(t, "cookie.Path", "/", cookie.Path)

	r = do(t, h, "/get", cookie)

	if r.status != http.StatusOK {
		t.Fatalf("Unexpected status: got %d, wanted %d", r.status, http.StatusOK)
	}
	if len(r.cookies) != 0 {
		t.Fatalf("Unexpected number of cookies: got %d, wanted 0", len(r.cookies))
	}

	assertEqual(t, "body", "v", r.body)
}

// newSessionApp builds a tiny app on top of a session manager: /put stores a
// value in the session and /get writes it back. The returned handler is wrapped
// in LoadAndSave, as the real router will be.
func newSessionApp(t *testing.T, db *sql.DB, secureCookies bool) http.Handler {
	t.Helper()

	sm, stop := NewSessionManager(db, secureCookies)
	t.Cleanup(stop)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /put", func(w http.ResponseWriter, r *http.Request) {
		sm.Put(r.Context(), "k", "v")
	})
	mux.HandleFunc("GET /get", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, sm.GetString(r.Context(), "k"))
	})
	return sm.LoadAndSave(mux)
}

// sessionCookie returns the cookie named session of a response.
func sessionCookie(t *testing.T, r response) *http.Cookie {
	t.Helper()

	for _, c := range r.cookies {
		if c.Name == "session" {
			return c
		}
	}
	t.Fatal("no cookie named session in the response")
	return nil
}

func TestSessionManager_secureCookie(t *testing.T) {
	app := newSessionApp(t, newTestDB(t), true)

	cookie := sessionCookie(t, do(t, app, "/put"))

	assertEqual(t, "cookie.Secure", true, cookie.Secure)
}

func TestSessionManager_noValidSession(t *testing.T) {
	app := newSessionApp(t, newTestDB(t), false)

	testCases := []struct {
		name    string
		cookies []*http.Cookie
	}{
		{name: "no cookie"},
		{name: "unknown token", cookies: []*http.Cookie{{Name: "session", Value: "not-a-real-token"}}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := do(t, app, "/get", tc.cookies...)

			assertEqual(t, "status", http.StatusOK, r.status)
			assertEqual(t, "body", "", r.body)
			assertEqual(t, "number of cookies", 0, len(r.cookies))
		})
	}
}

// TestSessionManager_sharedDatabase proves that sessions live in the database:
// a session created through one manager is found by another one that shares only
// the database, like two servers behind a load balancer.
func TestSessionManager_sharedDatabase(t *testing.T) {
	db := newTestDB(t)
	serverA := newSessionApp(t, db, false)
	serverB := newSessionApp(t, db, false)

	cookie := sessionCookie(t, do(t, serverA, "/put"))

	r := do(t, serverB, "/get", cookie)

	assertEqual(t, "body served by the other server", "v", r.body)
}
