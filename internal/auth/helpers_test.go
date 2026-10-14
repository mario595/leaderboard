package auth

import (
	"log/slog"
	"net/http"
	"testing"
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
