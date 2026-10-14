package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mario595/leaderboard/internal/auth"
)

// TestRouter_authRoutes checks that the login routes are registered when an auth
// handler is given, and absent when it is nil.
func TestRouter_authRoutes(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	// The user store is not needed for these requests: login only redirects, and
	// a callback without state is rejected before anything is saved.
	authHandler := auth.New(
		auth.NewOauthConfig("test-id", "test-secret", "http://localhost:8080"),
		nil, logger, http.DefaultClient, false,
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
			name:       "callback is registered and rejects a request without state",
			handler:    authHandler,
			method:     http.MethodGet,
			path:       "/auth/strava/callback",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "login only accepts GET",
			handler:    authHandler,
			method:     http.MethodPost,
			path:       "/auth/strava",
			wantStatus: http.StatusMethodNotAllowed,
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
			router := NewRouter(logger, tc.handler)
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
	router := NewRouter(logger, nil)
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
