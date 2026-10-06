package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewRouter(logger)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	got := rec.Header().Get("Content-Type")
	want := "application/json"

	if got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}

	var m map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Fatalf("Error decoding body, %v", err)
	}

	if m["status"] != "ok" {
		t.Errorf("Incorrect body, want 'ok', got %q", m["status"])
	}
}
