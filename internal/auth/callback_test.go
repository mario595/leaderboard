package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestCallbackValidations(t *testing.T) {
	testCases := []struct {
		name              string
		cookie            string
		urlValues         url.Values
		expectedStatus    int
		wantCookieCleared bool
	}{
		{
			name:           "user_declined",
			urlValues:      url.Values{"error": {"access_denied"}},
			expectedStatus: http.StatusForbidden,
		},
		{
			name: "no_cookie",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"activity:read,activity:read_all"},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "cookie_differs",
			cookie: "other",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"activity:read,activity:read_all"},
			},
			expectedStatus:    http.StatusBadRequest,
			wantCookieCleared: true,
		},
		{
			name:   "empty_state",
			cookie: "abc",
			urlValues: url.Values{
				"code":  {"x"},
				"scope": {"activity:read,activity:read_all"},
			},
			expectedStatus:    http.StatusBadRequest,
			wantCookieCleared: true,
		},
		{
			name:   "missing_code",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"scope": {"activity:read,activity:read_all"},
			},
			expectedStatus:    http.StatusBadRequest,
			wantCookieCleared: true,
		},
		{
			name:   "scope_read_only",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"read"},
			},
			expectedStatus:    http.StatusForbidden,
			wantCookieCleared: true,
		},
		{
			name:   "comma_scope_ok",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"read,activity:read"},
			},
			expectedStatus:    http.StatusSeeOther,
			wantCookieCleared: true,
		},
		{
			name:   "space_scope_ok",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"read activity:read"},
			},
			expectedStatus:    http.StatusSeeOther,
			wantCookieCleared: true,
		},
		{
			name:   "read_all_ok",
			cookie: "abc",
			urlValues: url.Values{
				"state": {"abc"},
				"code":  {"x"},
				"scope": {"read,activity:read_all"},
			},
			expectedStatus:    http.StatusSeeOther,
			wantCookieCleared: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
			h := newHandlerWith(fs, &fakeUserStore{})
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/strava/callback?"+tc.urlValues.Encode(), nil)

			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "oauth_state", Value: tc.cookie})
			}

			rec := httptest.NewRecorder()

			serveCallback(h, rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.expectedStatus)
			}

			// Only an accepted login creates a session.
			if c := sessionCookieOf(rec); c != nil && tc.expectedStatus != http.StatusSeeOther {
				t.Errorf("rejected login sets a session cookie %q, want none", c.Name)
			}

			if tc.wantCookieCleared {
				res := rec.Result()
				defer func() { _ = res.Body.Close() }()
				var found *http.Cookie
				for _, c := range res.Cookies() {
					if c.Name == "oauth_state" {
						found = c
					}
				}

				if found == nil {
					t.Fatalf("no oauth_state cookie in the response")
				}
				cookieMaxAge := found.MaxAge
				if cookieMaxAge >= 0 {
					t.Errorf("Cookie oauth_state MaxAge %d, expected -1", cookieMaxAge)
				}
			}

		})
	}

}

func TestCallbackHappyPath(t *testing.T) {
	fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
	users := &fakeUserStore{}
	h := newHandlerWith(fs, users)
	urlValues := url.Values{
		"state": {"abc"},
		"code":  {"x"},
		"scope": {"read,activity:read_all"}}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/strava/callback?"+urlValues.Encode(), nil)

	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "abc"})

	rec := httptest.NewRecorder()

	serveCallback(h, rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	if loc := rec.Result().Header.Get("Location"); loc != redirectTo {
		t.Errorf("Location: got %s, want %s", loc, redirectTo)
	}

	var cookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "session" {
			cookie = c
		}
	}
	if cookie == nil {
		t.Errorf("A session cookie was not found")
	}

	testutil.AssertEqual(t, "upsert calls", 1, users.calls)
	testutil.AssertEqual(t, "StravaID", int64(4242), users.got.StravaID)
	testutil.AssertEqual(t, "First Name", "Ada", users.got.FirstName)
	testutil.AssertEqual(t, "Last Name", "Lovelace", users.got.LastName)
	testutil.AssertEqual(t, "Access Token", "AT", users.got.AccessToken)
	testutil.AssertEqual(t, "Refresh Token", "RT", users.got.RefreshToken)
	if !users.got.TokenExpiresAt.After(time.Now().Add(5 * time.Hour)) {
		t.Errorf("Incorrect exipresAt")
	}

	testutil.AssertEqual(t, "code", "x", fs.form.Get("code"))
	testutil.AssertEqual(t, "code", "authorization_code", fs.form.Get("grant_type"))
	testutil.AssertEqual(t, "client_id", "test-id", fs.form.Get("client_id"))
	testutil.AssertEqual(t, "client_secret", "test-secret", fs.form.Get("client_secret"))

	who := h.sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, h.sessions.GetInt64(r.Context(), "userID"))
	}))

	req = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	whoRec := httptest.NewRecorder()
	who.ServeHTTP(whoRec, req)
	testutil.AssertEqual(t, "user id stored in the sesson", "1", whoRec.Body.String())
}

func serveCallback(h *Handler, w http.ResponseWriter, r *http.Request) {
	handler := h.sessions.LoadAndSave(http.HandlerFunc(h.Callback))
	handler.ServeHTTP(w, r)
}

// sessionCookieOf returns the session cookie set by a response, or nil.
func sessionCookieOf(rec *httptest.ResponseRecorder) *http.Cookie {
	res := rec.Result()
	defer func() { _ = res.Body.Close() }()

	for _, c := range res.Cookies() {
		if c.Name == "session" {
			return c
		}
	}
	return nil
}

// TestCallback_renewsExistingSession is the session fixation defence: a session
// that exists before the login (for instance one planted by an attacker) must
// not survive it. The login has to hand out a new token, and the old one must
// stop working.
func TestCallback_renewsExistingSession(t *testing.T) {
	fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
	h := newHandlerWith(fs, &fakeUserStore{})

	// A session that exists before the login.
	plant := h.sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.sessions.Put(r.Context(), "planted", "yes")
	}))
	plantRec := httptest.NewRecorder()
	plant.ServeHTTP(plantRec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	old := sessionCookieOf(plantRec)
	if old == nil {
		t.Fatal("setup: no session cookie for the existing session")
	}

	// Log in carrying that session.
	req := validCallbackRequest(t)
	req.AddCookie(old)
	rec := httptest.NewRecorder()
	serveCallback(h, rec, req)

	testutil.AssertEqual(t, "status", http.StatusSeeOther, rec.Code)
	fresh := sessionCookieOf(rec)
	if fresh == nil {
		t.Fatal("the login did not set a session cookie")
	}
	if fresh.Value == old.Value {
		t.Error("the login kept the session token that existed before it")
	}

	// The old token must no longer give access to the old session.
	read := h.sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, h.sessions.GetString(r.Context(), "planted"))
	}))
	oldReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	oldReq.AddCookie(old)
	oldRec := httptest.NewRecorder()
	read.ServeHTTP(oldRec, oldReq)
	testutil.AssertEqual(t, "data reachable with the old token", "", oldRec.Body.String())
}
