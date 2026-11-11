package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mario595/leaderboard/internal/auth"
	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

func TestMiddleware(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	normalRouter := NewRouter(Deps{Logger: logger, Sessions: scs.New()})

	panicRouter := chi.NewRouter()
	panicRouter.Use(middleware.RequestID)
	panicRouter.Use(requestLogger(logger))
	panicRouter.Use(middleware.Recoverer)

	panicRouter.Get("/boom", func(w http.ResponseWriter, r *http.Request) { panic("boom") })

	tests := []struct {
		name   string
		router http.Handler
		msg    string
		method string
		path   string
		status float64
	}{{
		name:   "Normal_request",
		router: normalRouter,
		msg:    "request",
		method: http.MethodGet,
		path:   "/healthz",
		status: http.StatusOK,
	},
		{
			name:   "Panic_request",
			router: panicRouter,
			msg:    "request",
			method: http.MethodGet,
			path:   "/boom",
			status: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf.Reset()
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.path, nil)
			res := httptest.NewRecorder()
			tc.router.ServeHTTP(res, req)
			out := decodeLogLine(t, &buf)

			wanted := "request"
			got := out["msg"]
			if got != wanted {
				t.Errorf("incorrect msg. wanted %v, got: %v", wanted, got)
			}

			wanted = tc.method
			got = out["method"]

			if got != wanted {
				t.Errorf("incorrect method. wanted %v, got: %v", wanted, got)
			}

			wanted = tc.path
			got = out["path"]

			if got != wanted {
				t.Errorf("incorrect path. wanted %v, got: %v", wanted, got)
			}

			if out["request_id"] == nil {
				t.Errorf("Missing request_id")
			}

			wantedStatus := float64(tc.status)
			gotStatus := out["status"]

			if gotStatus != wantedStatus {
				t.Errorf("incorrect status. wanted %v, got: %v", wantedStatus, gotStatus)
			}
		})
	}
}

func decodeLogLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("Error unmarshalling output, %v", err)
	}
	return out
}

// authApp is a small app built to test requireAuth: routes that create
// different kinds of sessions, and a protected route behind the middleware.
type authApp struct {
	handler         http.Handler
	protectedCalled bool
}

// newAuthApp builds the app with a real session manager (in-memory store).
//
//	/login      creates a session of a logged-in user (id 42)
//	/other      creates a session that holds something else
//	/protected  is behind requireAuth and writes the id it receives
func newAuthApp(t *testing.T) *authApp {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	sessions := scs.New()
	app := &authApp{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		sessions.Put(r.Context(), auth.SessionUserIDKey, int64(42))
	})
	mux.HandleFunc("GET /other", func(w http.ResponseWriter, r *http.Request) {
		sessions.Put(r.Context(), "something", "else")
	})
	protected := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.protectedCalled = true
		u, ok := userFromContext(r.Context())
		_, _ = fmt.Fprintf(w, "%d %t", u.ID, ok)
	})
	// The user of the session exists: these tests are about the session.
	users := &fakeUserStore{user: store.User{ID: 42}}
	mux.Handle("GET /protected", requireAuth(sessions, users, logger)(protected))

	app.handler = sessions.LoadAndSave(mux)
	return app
}

// do sends a request to the app and returns what the tests look at.
func (a *authApp) do(t *testing.T, path string, cookies ...*http.Cookie) (status int, body string, header http.Header, set []*http.Cookie) {
	t.Helper()
	return a.doCtx(t, t.Context(), path, cookies...)
}

// doCtx is do with the request's context given, for example an already
// cancelled one to act as a client that went away.
func (a *authApp) doCtx(t *testing.T, ctx context.Context, path string, cookies ...*http.Cookie) (status int, body string, header http.Header, set []*http.Cookie) {
	t.Helper()

	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	return res.StatusCode, rec.Body.String(), res.Header, res.Cookies()
}

// sessionCookie returns the session cookie a request obtained, or fails.
func sessionCookie(t *testing.T, cookies []*http.Cookie) *http.Cookie {
	t.Helper()

	for _, c := range cookies {
		if c.Name == "session" {
			return c
		}
	}
	t.Fatal("no session cookie in the response")
	return nil
}

func TestRequireAuth_withoutSession(t *testing.T) {
	app := newAuthApp(t)

	status, body, header, _ := app.do(t, "/protected")

	testutil.AssertEqual(t, "status", http.StatusUnauthorized, status)
	testutil.AssertEqual(t, "Content-Type", "application/json", header.Get("Content-Type"))

	var got map[string]string
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", body, err)
	}
	testutil.AssertEqual(t, "error message", "unauthorized", got["error"])

	if app.protectedCalled {
		t.Error("the protected handler ran for a request without a session")
	}
}

func TestRequireAuth_unknownSessionToken(t *testing.T) {
	app := newAuthApp(t)

	status, _, _, _ := app.do(t, "/protected", &http.Cookie{Name: "session", Value: "not-a-real-token"})

	testutil.AssertEqual(t, "status", http.StatusUnauthorized, status)
	if app.protectedCalled {
		t.Error("the protected handler ran for an unknown session token")
	}
}

func TestRequireAuth_sessionWithoutUser(t *testing.T) {
	app := newAuthApp(t)
	_, _, _, set := app.do(t, "/other")

	status, _, _, _ := app.do(t, "/protected", sessionCookie(t, set))

	testutil.AssertEqual(t, "status", http.StatusUnauthorized, status)
	if app.protectedCalled {
		t.Error("the protected handler ran for a session without a user")
	}
}

func TestRequireAuth_loggedInUser(t *testing.T) {
	app := newAuthApp(t)
	_, _, _, set := app.do(t, "/login")

	status, body, _, _ := app.do(t, "/protected", sessionCookie(t, set))

	testutil.AssertEqual(t, "status", http.StatusOK, status)
	// The handler received the id through the request context.
	testutil.AssertEqual(t, "id and ok seen by the handler", "42 true", body)
	if !app.protectedCalled {
		t.Error("the protected handler did not run for a logged-in user")
	}
}

func TestUserFromContext(t *testing.T) {
	// A key of another type, even with the same underlying shape, is a different key.
	type otherKey struct{}

	testCases := []struct {
		name   string
		ctx    context.Context
		wantID int64
		wantOK bool
	}{
		{name: "not set", ctx: context.Background()},
		{name: "set", ctx: context.WithValue(context.Background(), userKey{}, store.User{ID: 7}), wantID: 7, wantOK: true},
		// The value is a store.User, not a pointer to one and not the bare ID.
		{name: "pointer instead of value", ctx: context.WithValue(context.Background(), userKey{}, &store.User{ID: 7})},
		{name: "bare ID", ctx: context.WithValue(context.Background(), userKey{}, int64(7))},
		{name: "same value under another key", ctx: context.WithValue(context.Background(), otherKey{}, store.User{ID: 7})},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			u, ok := userFromContext(tc.ctx)

			testutil.AssertEqual(t, "id", tc.wantID, u.ID)
			testutil.AssertEqual(t, "ok", tc.wantOK, ok)
		})
	}
}
