package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/mario595/leaderboard/internal/auth"
	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
	"github.com/mario595/leaderboard/internal/webhook"
)

// newAPIApp builds the real router with a fake user store, plus a test-only
// route, /test-login, that creates a session for user 42. The router has no
// route that logs a user in without Strava, so the test creates the session
// through the same session manager the router uses.
func newAPIApp(t *testing.T, users userGetter) *authApp {
	t.Helper()
	return newAPIAppWithLogger(t, users, slog.New(slog.DiscardHandler))
}

// newAPIAppWithLogger is newAPIApp with the router's logger given, for tests
// that look at the log.
func newAPIAppWithLogger(t *testing.T, users userGetter, logger *slog.Logger) *authApp {
	t.Helper()

	sessions := scs.New()
	router := NewRouter(Deps{
		Logger:   logger,
		Sessions: sessions,
		Users:    users,
	})

	outer := http.NewServeMux()
	outer.Handle("GET /test-login", sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessions.Put(r.Context(), auth.SessionUserIDKey, int64(42))
	})))
	outer.Handle("/", router)
	return &authApp{handler: outer}
}

func TestRouter_apiRequiresLogin(t *testing.T) {
	app := newAPIApp(t, &fakeUserStore{})

	testCases := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "me without a session", path: "/api/me", wantStatus: http.StatusUnauthorized},
		// The middleware of a sub-router runs before routing, so an anonymous
		// visitor cannot find out which API paths exist.
		{name: "unknown API path without a session", path: "/api/nothing-here", wantStatus: http.StatusUnauthorized},
		{name: "health stays public", path: "/healthz", wantStatus: http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status, _, _, _ := app.do(t, tc.path)

			testutil.AssertEqual(t, "status", tc.wantStatus, status)
		})
	}
}

// A client that leaves while requireAuth looks up its user is not a server
// error: Debug, no Error line, and 499 in the request log. A real store
// failure still is (positive control).
func TestRouter_cancelledRequest(t *testing.T) {
	tests := []struct {
		name      string
		cancelled bool
		storeErr  error
		// What the request log and the response must show.
		wantStatus int
		wantErrors int // lines at ERROR
		wantDebug  int // "request canceled by client" lines at DEBUG
	}{
		{
			name:       "client_gone",
			cancelled:  true,
			wantStatus: 499,
			wantErrors: 0,
			wantDebug:  1,
		},
		{
			name:       "store_failure",
			cancelled:  false,
			storeErr:   errors.New("disk I/O error"),
			wantStatus: http.StatusInternalServerError,
			wantErrors: 1,
			wantDebug:  0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			users := &fakeUserStore{user: store.User{ID: 42}}
			app := newAPIAppWithLogger(t, users, logger)
			_, _, _, set := app.do(t, "/test-login")
			cookie := sessionCookie(t, set)
			// Only the /api/me request below goes into the log under test.
			buf.Reset()
			users.err = tc.storeErr

			ctx := t.Context()
			if tc.cancelled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			status, _, _, _ := app.doCtx(t, ctx, "/api/me", cookie)

			testutil.AssertEqual(t, "response status", tc.wantStatus, status)
			// The store was really asked: the 499 comes from its error, not
			// from a shortcut before the lookup.
			testutil.AssertEqual(t, "user lookups", 1, len(users.asked))

			var requestLines, errorLines, debugLines int
			for _, line := range logLines(t, &buf) {
				switch {
				case line["msg"] == "request":
					requestLines++
					testutil.AssertEqual[any](t, "request log status", float64(tc.wantStatus), line["status"])
				case line["level"] == "ERROR":
					errorLines++
					// The log says what failed; "internal error" is only the response.
					testutil.AssertEqual[any](t, "error msg", "look up session user", line["msg"])
					testutil.AssertEqual[any](t, "user_id", float64(42), line["user_id"])
				case line["level"] == "DEBUG" && line["msg"] == "request canceled by client":
					debugLines++
					testutil.AssertEqual[any](t, "user_id", float64(42), line["user_id"])
				}
			}
			testutil.AssertEqual(t, "request log lines", 1, requestLines)
			testutil.AssertEqual(t, "error lines", tc.wantErrors, errorLines)
			testutil.AssertEqual(t, "debug lines", tc.wantDebug, debugLines)
		})
	}
}

func TestRouter_apiMeForALoggedInUser(t *testing.T) {
	users := &fakeUserStore{user: store.User{
		ID: 42, StravaID: 4242, FirstName: "Ada", LastName: "Lovelace",
		AccessToken: "accessToken", RefreshToken: "refreshToken",
	}}
	app := newAPIApp(t, users)
	_, _, _, set := app.do(t, "/test-login")
	cookie := sessionCookie(t, set)

	status, body, _, _ := app.do(t, "/api/me", cookie)

	testutil.AssertEqual(t, "status", http.StatusOK, status)
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", body, err)
	}
	testutil.AssertEqual[any](t, "firstName", "Ada", got["firstName"])

	// Logged in, an unknown API path is a plain 404.
	status, _, _, _ = app.do(t, "/api/nothing-here", cookie)
	testutil.AssertEqual(t, "status of an unknown API path", http.StatusNotFound, status)
}

func TestRouter_sessionOfDeletedUser(t *testing.T) {
	users := &fakeUserStore{user: store.User{ID: 42, StravaID: 4242, FirstName: "Ada"}}
	app := newAPIApp(t, users)
	_, _, _, set := app.do(t, "/test-login")
	cookie := sessionCookie(t, set)

	// Positive control: the session works while the user exists.
	status, _, _, _ := app.do(t, "/api/me", cookie)
	testutil.AssertEqual(t, "status before the deletion", http.StatusOK, status)

	// The user deauthorizes and is deleted (#15). Every API path, not only
	// /api/me, must stop serving them: an unknown path gives 401, not 404.
	users.err = store.ErrNotFound
	status, _, _, set = app.do(t, "/api/nothing-here", cookie)
	testutil.AssertEqual(t, "status after the deletion", http.StatusUnauthorized, status)

	// The session is destroyed, not just refused: the browser is told to
	// drop the cookie...
	if c := sessionCookie(t, set); c.MaxAge >= 0 && c.Expires.After(time.Now()) {
		t.Errorf("session cookie not cleared: %+v", c)
	}
	// ...and the old token no longer works, even if the user came back.
	users.err = nil
	status, _, _, _ = app.do(t, "/api/me", cookie)
	testutil.AssertEqual(t, "status with the destroyed session", http.StatusUnauthorized, status)
}

func TestRouter_userStoreErrorIsNot401(t *testing.T) {
	users := &fakeUserStore{user: store.User{ID: 42, StravaID: 4242, FirstName: "Ada"}}
	app := newAPIApp(t, users)
	_, _, _, set := app.do(t, "/test-login")
	cookie := sessionCookie(t, set)

	// A database outage must not look like a logout.
	users.err = errors.New("database is locked")
	status, _, _, _ := app.do(t, "/api/nothing-here", cookie)
	testutil.AssertEqual(t, "status during the outage", http.StatusInternalServerError, status)

	// The session survived the outage.
	users.err = nil
	status, _, _, _ = app.do(t, "/api/me", cookie)
	testutil.AssertEqual(t, "status after the outage", http.StatusOK, status)
}

func TestRouter_leaderboard(t *testing.T) {
	users := &fakeUserStore{user: store.User{ID: 42, FirstName: "Ada"}}
	sessions := scs.New()
	router := NewRouter(Deps{
		Logger:    slog.New(slog.DiscardHandler),
		Sessions:  sessions,
		Users:     users,
		Seasons:   &fakeSeasons{current: store.Season{ID: 3, Name: "Autumn 2026"}},
		Standings: &fakeStandings{rows: []store.Standing{{UserID: 42, FirstName: "Ada", Points: 6}}},
	})
	outer := http.NewServeMux()
	outer.Handle("GET /test-login", sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessions.Put(r.Context(), auth.SessionUserIDKey, int64(42))
	})))
	outer.Handle("/", router)
	app := &authApp{handler: outer}

	// Behind requireAuth: anonymous visitors get 401.
	status, _, _, _ := app.do(t, "/api/leaderboard")
	testutil.AssertEqual(t, "status without a session", http.StatusUnauthorized, status)

	_, _, _, set := app.do(t, "/test-login")
	status, body, _, _ := app.do(t, "/api/leaderboard", sessionCookie(t, set))
	testutil.AssertEqual(t, "status with a session", http.StatusOK, status)
	if !strings.Contains(body, `"points":6`) {
		t.Errorf("body %s does not contain the standings", body)
	}
}

func TestRouter_myActivities(t *testing.T) {
	users := &fakeUserStore{user: store.User{ID: 42, FirstName: "Ada"}}
	lister := &fakeLister{rows: []store.Activity{}}
	sessions := scs.New()
	router := NewRouter(Deps{
		Logger:     slog.New(slog.DiscardHandler),
		Sessions:   sessions,
		Users:      users,
		Seasons:    &fakeSeasons{current: store.Season{ID: 3, Name: "Autumn 2026"}},
		Standings:  &fakeStandings{},
		Activities: lister,
	})
	outer := http.NewServeMux()
	outer.Handle("GET /test-login", sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessions.Put(r.Context(), auth.SessionUserIDKey, int64(42))
	})))
	outer.Handle("/", router)
	app := &authApp{handler: outer}

	status, _, _, _ := app.do(t, "/api/me/activities")
	testutil.AssertEqual(t, "status without a session", http.StatusUnauthorized, status)

	_, _, _, set := app.do(t, "/test-login")
	status, _, _, _ = app.do(t, "/api/me/activities", sessionCookie(t, set))
	testutil.AssertEqual(t, "status with a session", http.StatusOK, status)
	// Always the session's user: there is no parameter to ask for another.
	testutil.AssertEqual(t, "user listed", int64(42), lister.userID)
}

func TestRouter_noAPIWithoutUserStore(t *testing.T) {
	app := newAPIApp(t, nil)

	status, _, _, _ := app.do(t, "/api/me")

	testutil.AssertEqual(t, "status", http.StatusNotFound, status)
}

// TestRouter_authRoutes checks that the login routes are registered when an auth
// handler is given, and absent when it is nil.
func TestRouter_authRoutes(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	sessions := scs.New()
	// The user store is not needed for these requests: login only redirects, and
	// a callback without state is rejected before anything is saved.
	authHandler := auth.New(
		auth.NewOauthConfig("test-id", "test-secret", "http://localhost:8080"),
		nil, nil, sessions, logger, http.DefaultClient, false,
	)

	tests := []struct {
		name         string
		handler      *auth.Handler
		method       string
		path         string
		wantStatus   int
		wantLocation string // host of the redirect, if any
	}{
		{
			name:         "login redirects to Strava",
			handler:      authHandler,
			method:       http.MethodGet,
			path:         "/auth/strava",
			wantStatus:   http.StatusFound,
			wantLocation: "www.strava.com",
		},
		{
			// Rejected, but as a redirect back to the login page (#18).
			name:       "callback is registered and sends a request without state back to the app",
			handler:    authHandler,
			method:     http.MethodGet,
			path:       "/auth/strava/callback",
			wantStatus: http.StatusSeeOther,
		},
		{
			// The invitation link stores the code and starts the login.
			name:       "invitation link is registered",
			handler:    authHandler,
			method:     http.MethodGet,
			path:       "/invite/ABC234",
			wantStatus: http.StatusSeeOther,
		},
		{
			name:       "login only accepts GET",
			handler:    authHandler,
			method:     http.MethodPost,
			path:       "/auth/strava",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "logout only accepts POST",
			handler:    authHandler,
			method:     http.MethodGet,
			path:       "/auth/logout",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "logout",
			handler:    authHandler,
			method:     http.MethodPost,
			path:       "/auth/logout",
			wantStatus: http.StatusNoContent,
		},
		{
			name:       "no auth routes without an auth handler",
			handler:    nil,
			method:     http.MethodGet,
			path:       "/auth/strava",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouter(Deps{
				Logger:   logger,
				Auth:     tc.handler,
				Sessions: sessions,
			})
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			res := rec.Result()
			defer func() { _ = res.Body.Close() }()

			if res.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", res.StatusCode, tc.wantStatus)
			}
			if tc.wantLocation != "" {
				loc, err := res.Location()
				if err != nil {
					t.Fatalf("Location(): %v", err)
				}
				if loc.Host != tc.wantLocation {
					t.Errorf("redirect host = %q, want %q", loc.Host, tc.wantLocation)
				}
			}
		})
	}
}

func TestRouter_webhookRoutes(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	events := &fakeEventInserter{}
	router := NewRouter(Deps{
		Logger:   logger,
		Sessions: scs.New(),
		Users:    &fakeUserStore{},
		Webhook:  webhook.New(events, "verify-me", 120475, nil, logger),
	})

	const event = `{"aspect_type":"create","event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":120475}`
	testCases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		// Public: Strava has no session.
		{name: "handshake", method: http.MethodGet, path: "/webhook/strava?hub.mode=subscribe&hub.verify_token=verify-me&hub.challenge=abc", wantStatus: http.StatusOK},
		{name: "event", method: http.MethodPost, path: "/webhook/strava", body: event, wantStatus: http.StatusOK},
		// Positive control: the routes are not public by accident of an open router.
		{name: "api still protected", method: http.MethodGet, path: "/api/me", wantStatus: http.StatusUnauthorized},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			testutil.AssertEqual(t, "status", tc.wantStatus, rec.Code)
		})
	}
	testutil.AssertEqual(t, "events stored through the router", 1, events.count())
}

func TestRouter_noWebhookWithoutHandler(t *testing.T) {
	router := NewRouter(Deps{Logger: slog.New(slog.DiscardHandler), Sessions: scs.New()})

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/webhook/strava", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	testutil.AssertEqual(t, "status", http.StatusNotFound, rec.Code)
}

// fakeEventInserter counts stored events.
type fakeEventInserter struct {
	mu sync.Mutex
	n  int
}

func (f *fakeEventInserter) Insert(_ context.Context, _ store.Event) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	return true, nil
}

func (f *fakeEventInserter) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.n
}

func TestRouter(t *testing.T) {

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "GET foo",
			method:     http.MethodGet,
			path:       "/foo",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "POST_healthz",
			method:     http.MethodPost,
			path:       "/healthz",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "PUT_healthz",
			method:     http.MethodPut,
			path:       "/healthz",
			wantStatus: http.StatusMethodNotAllowed,
		},
	}

	logger := slog.New(slog.DiscardHandler)
	router := NewRouter(Deps{Logger: logger, Sessions: scs.New()})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("Error in %q. Incorrect statusCode. got %d. Wanted %d", tc.name, rec.Code, tc.wantStatus)
			}
		})
	}

}

// TestRouter_spa checks that every path the server does not handle itself
// reaches the frontend, and that the frontend never shadows the API, the
// webhook or the health check.
func TestRouter_spa(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	sessions := scs.New()
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("spa"))
	})
	router := NewRouter(Deps{
		Logger:   logger,
		Sessions: sessions,
		Users:    &fakeUserStore{user: store.User{ID: 42, FirstName: "Ada"}},
		Webhook:  webhook.New(&fakeEventInserter{}, "verify-me", 120475, nil, logger),
		SPA:      spa,
	})
	outer := http.NewServeMux()
	outer.Handle("GET /test-login", sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessions.Put(r.Context(), auth.SessionUserIDKey, int64(42))
	})))
	outer.Handle("/", router)
	app := &authApp{handler: outer}
	_, _, _, set := app.do(t, "/test-login")
	cookie := sessionCookie(t, set)

	testCases := []struct {
		name       string
		method     string
		path       string
		loggedIn   bool
		wantStatus int
		wantSPA    bool
	}{
		{name: "root", path: "/", wantStatus: http.StatusOK, wantSPA: true},
		{name: "client route", path: "/activities", wantStatus: http.StatusOK, wantSPA: true},
		{name: "nested client route", path: "/a/b/c", wantStatus: http.StatusOK, wantSPA: true},
		// The recorder keeps the body even for HEAD (a real server drops it).
		{name: "HEAD", method: http.MethodHead, path: "/rules", wantStatus: http.StatusOK, wantSPA: true},
		{name: "not for POST", method: http.MethodPost, path: "/activities", wantStatus: http.StatusMethodNotAllowed},
		{name: "health", path: "/healthz", wantStatus: http.StatusOK},
		{name: "unknown API path, anonymous", path: "/api/nothing-here", wantStatus: http.StatusUnauthorized},
		{name: "unknown API path, logged in", path: "/api/nothing-here", loggedIn: true, wantStatus: http.StatusNotFound},
		{name: "API still works", path: "/api/me", loggedIn: true, wantStatus: http.StatusOK},
		{name: "unknown webhook path", path: "/webhook/nothing-here", wantStatus: http.StatusNotFound},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			method := tc.method
			if method == "" {
				method = http.MethodGet
			}
			req := httptest.NewRequestWithContext(t.Context(), method, tc.path, nil)
			if tc.loggedIn {
				req.AddCookie(cookie)
			}
			rec := httptest.NewRecorder()

			app.handler.ServeHTTP(rec, req)

			testutil.AssertEqual(t, "status", tc.wantStatus, rec.Code)
			testutil.AssertEqual(t, "served by the SPA", tc.wantSPA, rec.Body.String() == "spa")
		})
	}
}
