package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/mario595/leaderboard/internal/testutil"
)

func TestInvite(t *testing.T) {
	h := newTestHandler(false)

	rec := serveInvite(t, h, "ABC234")

	// See Other: the browser follows with a GET to the login.
	testutil.AssertEqual(t, "status", http.StatusSeeOther, rec.Code)
	testutil.AssertEqual(t, "Location", "/auth/strava", rec.Result().Header.Get("Location"))
	session := sessionCookieOf(rec)
	if session == nil {
		t.Fatal("no session cookie: the code must survive the trip to Strava")
	}
	testutil.AssertEqual(t, "code in the session", "ABC234", sessionString(t, h, session, invitationKey))
}

// A second link replaces the first: the code that counts is the last one
// opened.
func TestInvite_lastLinkWins(t *testing.T) {
	h := newTestHandler(false)
	first := sessionCookieOf(serveInvite(t, h, "FIRST"))

	rec := serveInvite(t, h, "SECOND", first)

	session := sessionCookieOf(rec)
	if session == nil {
		session = first // the token did not change
	}
	testutil.AssertEqual(t, "code in the session", "SECOND", sessionString(t, h, session, invitationKey))
}

// serveInvite serves GET /invite/{code} the way the router does: through chi,
// so chi.URLParam works, and the session middleware.
func serveInvite(t *testing.T, h *Handler, code string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Use(h.sessions.LoadAndSave)
	r.Get("/invite/{code}", h.Invite)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/invite/"+code, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// sessionString reads a string from the session that the cookie points to.
func sessionString(t *testing.T, h *Handler, session *http.Cookie, key string) string {
	t.Helper()
	read := h.sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, h.sessions.GetString(r.Context(), key))
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(session)
	rec := httptest.NewRecorder()
	read.ServeHTTP(rec, req)
	return rec.Body.String()
}

// sessionUserID reads the logged-in user's ID from the session (0 if none).
func sessionUserID(t *testing.T, h *Handler, session *http.Cookie) int64 {
	t.Helper()
	var id int64
	read := h.sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id = h.sessions.GetInt64(r.Context(), SessionUserIDKey)
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(session)
	read.ServeHTTP(httptest.NewRecorder(), req)
	return id
}
