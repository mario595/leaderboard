package strava

import (
	"errors"
	"net/http"
	"testing"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestCheckAccess_accepted(t *testing.T) {
	fake := newFakeStrava(t)
	client := newTestAPI(fake).ForUser(7, validToken())

	if err := client.CheckAccess(t.Context()); err != nil {
		t.Fatalf("CheckAccess: %v", err)
	}

	testutil.AssertEqual(t, "API calls", 1, fake.apiCount())
	testutil.AssertEqual(t, "path", "/athlete", fake.lastAPIPath())
}

func TestCheckAccess_unauthorized(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*fakeStrava)
		expired bool // start from an expired token, so the refresh runs
	}{
		{name: "401 from the API", setup: func(f *fakeStrava) { f.apiStatus = http.StatusUnauthorized }},
		{name: "refresh rejected with 400", setup: func(f *fakeStrava) { f.tokenStatus = http.StatusBadRequest }, expired: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeStrava(t)
			tt.setup(fake)
			tok := validToken()
			if tt.expired {
				tok = expiredToken()
			}
			client := newTestAPI(fake).ForUser(7, tok)

			err := client.CheckAccess(t.Context())

			if !errors.Is(err, ErrUnauthorized) {
				t.Fatalf("want ErrUnauthorized, got %v", err)
			}
		})
	}
}

// Positive control: an error that says nothing about the authorization must
// not look like a revoke, or prune-revoked would delete a member.
func TestCheckAccess_otherErrorsAreNotUnauthorized(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*fakeStrava)
	}{
		{name: "500 from the API", setup: func(f *fakeStrava) { f.apiStatus = http.StatusInternalServerError }},
		{name: "429 from the API", setup: func(f *fakeStrava) { f.apiStatus = http.StatusTooManyRequests }},
		{name: "500 from the token endpoint", setup: func(f *fakeStrava) { f.tokenStatus = http.StatusInternalServerError }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := newFakeStrava(t)
			tt.setup(fake)
			client := newTestAPI(fake).ForUser(7, expiredToken())

			err := client.CheckAccess(t.Context())

			if err == nil {
				t.Fatal("got nil error")
			}
			if errors.Is(err, ErrUnauthorized) {
				t.Errorf("want an error that is not ErrUnauthorized, got %v", err)
			}
		})
	}
}
