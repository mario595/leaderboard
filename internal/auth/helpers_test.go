package auth

import (
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mario595/leaderboard/internal/store"
)

func assertEqual[T comparable](t *testing.T, name string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("unexpected value %s: wanted %v, got %v", name, want, got)
	}
}

func newTestHandler(secureCookies bool) *Handler {
	cfg := NewOauthConfig("test-id", "test-secret", "http://localhost:8080")
	return New(cfg, nil, slog.New(slog.DiscardHandler), http.DefaultClient, secureCookies)
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	if err = store.Migrate(t.Context(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return db
}

type response struct {
	status  int
	body    string
	cookies []*http.Cookie
}

func do(t *testing.T, h http.Handler, path string, cookies ...*http.Cookie) response {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	res := rec.Result()
	defer func() { _ = res.Body.Close() }()
	return response{status: res.StatusCode, body: rec.Body.String(), cookies: res.Cookies()}
}
