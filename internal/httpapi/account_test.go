package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"

	"github.com/mario595/leaderboard/internal/auth"
	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

// Compile-time check: the real store deletes accounts.
var _ userDeleter = (*store.UserStore)(nil)

func TestDeleteAccount(t *testing.T) {
	app := newAccountApp(t)
	cookie := app.login(t, 42)
	otherDevice := app.login(t, 42)
	otherMember := app.login(t, 7)

	rec := app.do(t, http.MethodDelete, "/api/me", cookie)

	testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
	// Revoked and deleted: the member from the session, nobody else.
	testutil.AssertEqual(t, "revoked users", "[42]", app.revoked())
	testutil.AssertEqual(t, "deleted athletes", "[4242]", app.members.deletedIDs())

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body %q is not JSON: %v", rec.Body.String(), err)
	}
	testutil.AssertEqual(t, "deletedAt", "2026-11-11T15:04:05Z", body["deletedAt"])
	// 30 days: the longest any backup lives (manual copies; snapshots 14).
	testutil.AssertEqual(t, "backupsGoneBy", "2026-12-11", body["backupsGoneBy"])
	if strings.Contains(rec.Body.String(), "access") {
		t.Errorf("body %q mentions tokens", rec.Body.String())
	}

	// The response expires this browser's cookie.
	if c := sessionCookieIn(rec.Result().Cookies()); c == nil || c.MaxAge >= 0 {
		t.Errorf("session cookie not expired: %+v", c)
	}
	// Every session of the member is gone from the store, on every device.
	// Counted before any further request: requireAuth would also destroy a
	// leftover session on its next use, which would hide a missing cleanup.
	testutil.AssertEqual(t, "member sessions in the store", 0, app.sessionsOf(t, 42))
	testutil.AssertEqual(t, "other member sessions in the store", 1, app.sessionsOf(t, 7))
	testutil.AssertEqual(t, "this session after", http.StatusUnauthorized, app.do(t, http.MethodGet, "/api/me", cookie).Code)
	testutil.AssertEqual(t, "other device after", http.StatusUnauthorized, app.do(t, http.MethodGet, "/api/me", otherDevice).Code)
	// Another member is untouched (positive control).
	testutil.AssertEqual(t, "other member after", http.StatusOK, app.do(t, http.MethodGet, "/api/me", otherMember).Code)
}

// Whoever is logged in is the one deleted: the ID comes from the session.
func TestDeleteAccount_deletesTheSessionMember(t *testing.T) {
	app := newAccountApp(t)
	member42 := app.login(t, 42)
	cookie := app.login(t, 7)

	rec := app.do(t, http.MethodDelete, "/api/me", cookie)

	testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
	testutil.AssertEqual(t, "revoked users", "[7]", app.revoked())
	testutil.AssertEqual(t, "deleted athletes", "[707]", app.members.deletedIDs())
	testutil.AssertEqual(t, "member 42 after", http.StatusOK, app.do(t, http.MethodGet, "/api/me", member42).Code)
}

// The member asked to leave: a revoke that fails (network, already revoked
// on Strava) must not keep their data.
func TestDeleteAccount_revokeFails(t *testing.T) {
	app := newAccountApp(t)
	app.revokeErr = errors.New("strava deauthorize: status 500")
	cookie := app.login(t, 42)

	rec := app.do(t, http.MethodDelete, "/api/me", cookie)

	testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
	testutil.AssertEqual(t, "deleted athletes", "[4242]", app.members.deletedIDs())
	warn := app.logLine(t, "WARN")
	if warn == nil {
		t.Fatalf("no WARN line for the failed revoke: %s", app.logs.String())
	}
	testutil.AssertEqual[any](t, "user_id", float64(42), warn["user_id"])
	for _, secret := range []string{"access-42", "refresh-42"} {
		if strings.Contains(app.logs.String(), secret) {
			t.Errorf("log contains the token %q", secret)
		}
	}
}

func TestDeleteAccount_storeFails(t *testing.T) {
	app := newAccountApp(t)
	app.members.deleteErr = errors.New("database is locked")
	cookie := app.login(t, 42)

	rec := app.do(t, http.MethodDelete, "/api/me", cookie)

	testutil.AssertEqual(t, "status", http.StatusInternalServerError, rec.Code)
	if strings.Contains(rec.Body.String(), "locked") {
		t.Errorf("body %q leaks the internal error", rec.Body.String())
	}
	if app.logLine(t, "ERROR") == nil {
		t.Errorf("no ERROR line: %s", app.logs.String())
	}
	// Nothing was deleted, so the member is still logged in.
	testutil.AssertEqual(t, "session after a failed delete", http.StatusOK, app.do(t, http.MethodGet, "/api/me", cookie).Code)
}

// The work runs to the end even if the browser goes away halfway: a revoke
// without the delete would leave the member half gone.
func TestDeleteAccount_notCutShortByTheClient(t *testing.T) {
	app := newAccountApp(t)
	cookie := app.login(t, 42)
	app.members.honourContext = true
	app.revokeHook = func() { app.cancel() } // the browser leaves during the revoke

	ctx, cancel := context.WithCancel(t.Context())
	app.cancel = cancel
	req := httptest.NewRequestWithContext(ctx, http.MethodDelete, "/api/me", nil)
	req.AddCookie(cookie)
	app.handler.ServeHTTP(httptest.NewRecorder(), req)

	testutil.AssertEqual(t, "deleted athletes", "[4242]", app.members.deletedIDs())
}

// accountApp is DELETE /api/me and GET /api/me behind the real requireAuth
// and session manager (in-memory store, which can list its sessions), with a
// fixed clock, a fake revoke and a fake member store.
type accountApp struct {
	handler  http.Handler
	sessions *scs.SessionManager
	members  *fakeMembers
	logs     *bytes.Buffer

	mu         sync.Mutex
	revokedIDs []int64
	revokeErr  error
	revokeHook func()
	cancel     context.CancelFunc
}

func newAccountApp(t *testing.T) *accountApp {
	t.Helper()
	app := &accountApp{
		sessions: scs.New(),
		members:  newFakeMembers(42, 7),
		logs:     &bytes.Buffer{},
	}
	logger := slog.New(slog.NewJSONHandler(app.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	now := func() time.Time { return time.Date(2026, 11, 11, 16, 4, 5, 0, time.FixedZone("CET", 3600)) }
	revoke := func(_ context.Context, u store.User) error {
		app.mu.Lock()
		app.revokedIDs = append(app.revokedIDs, u.ID)
		hook, err := app.revokeHook, app.revokeErr
		app.mu.Unlock()
		if hook != nil {
			hook()
		}
		return err
	}

	guard := requireAuth(app.sessions, app.members, logger)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /test-login", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.ParseInt(r.URL.Query().Get("user"), 10, 64)
		app.sessions.Put(r.Context(), auth.SessionUserIDKey, id)
	})
	mux.Handle("GET /api/me", guard(meHandler(logger)))
	mux.Handle("DELETE /api/me", guard(deleteAccountHandler(revoke, app.members, app.sessions, now, logger)))
	app.handler = app.sessions.LoadAndSave(mux)
	return app
}

// login creates a session for the user, as a browser would, and returns its
// cookie.
func (a *accountApp) login(t *testing.T, userID int64) *http.Cookie {
	t.Helper()
	rec := a.do(t, http.MethodGet, "/test-login?user="+strconv.FormatInt(userID, 10))
	c := sessionCookieIn(rec.Result().Cookies())
	if c == nil {
		t.Fatal("no session cookie from /test-login")
	}
	return c
}

func (a *accountApp) do(t *testing.T, method, path string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

func (a *accountApp) revoked() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return formatIDs(a.revokedIDs)
}

// sessionsOf counts the stored sessions that belong to the user.
func (a *accountApp) sessionsOf(t *testing.T, userID int64) int {
	t.Helper()
	n := 0
	err := a.sessions.Iterate(t.Context(), func(ctx context.Context) error {
		if a.sessions.GetInt64(ctx, auth.SessionUserIDKey) == userID {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("iterate sessions: %v", err)
	}
	return n
}

// logLine returns the first log line at level, nil if none.
func (a *accountApp) logLine(t *testing.T, level string) map[string]any {
	t.Helper()
	for _, line := range logLines(t, a.logs) {
		if line["level"] == level {
			return line
		}
	}
	return nil
}

// fakeMembers is a user store with members whose Strava ID is their ID
// times 101 (42 -> 4242, 7 -> 707). DeleteByStravaID removes them, so
// requireAuth finds them gone afterwards.
type fakeMembers struct {
	mu            sync.Mutex
	users         map[int64]store.User
	deleted       []int64
	deleteErr     error
	honourContext bool // fail like database/sql on a done context
}

func newFakeMembers(ids ...int64) *fakeMembers {
	f := &fakeMembers{users: map[int64]store.User{}}
	for _, id := range ids {
		f.users[id] = store.User{
			ID: id, StravaID: id * 101, FirstName: "Member",
			AccessToken: "access-" + strconv.FormatInt(id, 10), RefreshToken: "refresh-" + strconv.FormatInt(id, 10),
		}
	}
	return f
}

func (f *fakeMembers) GetByID(_ context.Context, id int64) (store.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	u, ok := f.users[id]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeMembers) DeleteByStravaID(ctx context.Context, stravaID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.honourContext && ctx.Err() != nil {
		return false, ctx.Err()
	}
	if f.deleteErr != nil {
		return false, f.deleteErr
	}
	f.deleted = append(f.deleted, stravaID)
	for id, u := range f.users {
		if u.StravaID == stravaID {
			delete(f.users, id)
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeMembers) deletedIDs() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return formatIDs(f.deleted)
}

func formatIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func sessionCookieIn(cookies []*http.Cookie) *http.Cookie {
	for _, c := range cookies {
		if c.Name == "session" {
			return c
		}
	}
	return nil
}

// The route as wired by NewRouter: behind requireAuth, DELETE only, and
// absent when its dependencies are not given.
func TestRouter_deleteAccount(t *testing.T) {
	newApp := func(withAccounts bool) (*authApp, *fakeMembers) {
		members := newFakeMembers(42)
		sessions := scs.New()
		deps := Deps{Logger: slog.New(slog.DiscardHandler), Sessions: sessions, Users: members}
		if withAccounts {
			deps.Accounts = members
			deps.Revoke = func(context.Context, store.User) error { return nil }
		}
		outer := http.NewServeMux()
		outer.Handle("GET /test-login", sessions.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessions.Put(r.Context(), auth.SessionUserIDKey, int64(42))
		})))
		outer.Handle("/", NewRouter(deps))
		return &authApp{handler: outer}, members
	}
	send := func(t *testing.T, app *authApp, method string, cookie *http.Cookie) int {
		t.Helper()
		req := httptest.NewRequestWithContext(t.Context(), method, "/api/me", nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		app.handler.ServeHTTP(rec, req)
		return rec.Code
	}

	t.Run("anonymous", func(t *testing.T) {
		app, members := newApp(true)
		testutil.AssertEqual(t, "status", http.StatusUnauthorized, send(t, app, http.MethodDelete, nil))
		testutil.AssertEqual(t, "deleted", "[]", members.deletedIDs())
	})
	t.Run("POST is not a delete", func(t *testing.T) {
		app, members := newApp(true)
		_, _, _, set := app.do(t, "/test-login")
		testutil.AssertEqual(t, "status", http.StatusMethodNotAllowed, send(t, app, http.MethodPost, sessionCookie(t, set)))
		testutil.AssertEqual(t, "deleted", "[]", members.deletedIDs())
	})
	t.Run("DELETE is wired", func(t *testing.T) {
		app, members := newApp(true)
		_, _, _, set := app.do(t, "/test-login")
		testutil.AssertEqual(t, "status", http.StatusOK, send(t, app, http.MethodDelete, sessionCookie(t, set)))
		testutil.AssertEqual(t, "deleted", "[4242]", members.deletedIDs())
	})
	t.Run("not registered without its dependencies", func(t *testing.T) {
		app, members := newApp(false)
		_, _, _, set := app.do(t, "/test-login")
		testutil.AssertEqual(t, "status", http.StatusMethodNotAllowed, send(t, app, http.MethodDelete, sessionCookie(t, set)))
		testutil.AssertEqual(t, "deleted", "[]", members.deletedIDs())
	})
}
