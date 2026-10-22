package strava

import (
	"errors"
	"net/http"
	"testing"

	"golang.org/x/oauth2"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestErrors_notFound(t *testing.T) {
	fake := newFakeStrava(t)
	client := newTestAPI(fake).ForUser(7, validToken())

	// The fake answers 404 for IDs it has no fixture for, like Strava for a
	// deleted or private activity.
	_, err := client.Activity(t.Context(), 404)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Activity for an unknown ID: got error %v, wanted ErrNotFound", err)
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Errorf("Activity for an unknown ID: error %v must not be ErrUnauthorized", err)
	}
}

func TestErrors_unauthorized(t *testing.T) {
	fake := newFakeStrava(t)
	fake.apiStatus = http.StatusUnauthorized
	client := newTestAPI(fake).ForUser(7, validToken())

	_, err := client.Activity(t.Context(), 12345678901)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("Activity with status 401: got error %v, wanted ErrUnauthorized", err)
	}
}

func TestErrors_refreshRejectedIsUnauthorized(t *testing.T) {
	fake := newFakeStrava(t)
	fake.tokenStatus = http.StatusBadRequest // refresh token revoked by the user
	client := newTestAPI(fake).ForUser(7, expiredToken())

	_, err := client.Activity(t.Context(), 12345678901)
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("Activity with a rejected refresh: got error %v, wanted ErrUnauthorized", err)
	}
	// Both branches of the wrapped error stay reachable.
	var retrieveErr *oauth2.RetrieveError
	if !errors.As(err, &retrieveErr) {
		t.Errorf("Activity with a rejected refresh: error %v no longer wraps *oauth2.RetrieveError", err)
	}
}

func TestErrors_refreshServerErrorIsNotUnauthorized(t *testing.T) {
	fake := newFakeStrava(t)
	fake.tokenStatus = http.StatusInternalServerError // Strava hiccup: retry, do not log the user out
	client := newTestAPI(fake).ForUser(7, expiredToken())

	_, err := client.Activity(t.Context(), 12345678901)
	if err == nil {
		t.Fatal("Activity with a failing token endpoint: got nil error")
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Errorf("Activity with a 500 from the token endpoint: error %v must not be ErrUnauthorized", err)
	}
}

func TestErrors_rateLimited(t *testing.T) {
	testCases := []struct {
		name  string
		usage string
		want  RateLimitError
	}{
		{
			name:  "headers parsed",
			usage: "201,1803",
			want:  RateLimitError{ShortUsage: 201, ShortLimit: 200, DailyUsage: 1803, DailyLimit: 2000},
		},
		{
			// The 429 is the signal; bad numbers must not hide it.
			name:  "malformed usage header",
			usage: "lots",
			want:  RateLimitError{ShortUsage: 0, ShortLimit: 200, DailyUsage: 0, DailyLimit: 2000},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeStrava(t)
			fake.apiStatus = http.StatusTooManyRequests
			fake.rateUsage = tc.usage
			client := newTestAPI(fake).ForUser(7, validToken())

			_, err := client.Activity(t.Context(), 12345678901)

			var rateErr *RateLimitError
			if !errors.As(err, &rateErr) {
				t.Fatalf("Activity with status 429: got error %v, wanted a *RateLimitError", err)
			}
			if *rateErr != tc.want {
				t.Errorf("RateLimitError: got %+v, wanted %+v", *rateErr, tc.want)
			}
		})
	}
}

func TestErrors_otherStatusIsNotTyped(t *testing.T) {
	fake := newFakeStrava(t)
	fake.apiStatus = http.StatusInternalServerError
	client := newTestAPI(fake).ForUser(7, validToken())

	_, err := client.Activity(t.Context(), 12345678901)
	if err == nil {
		t.Fatal("Activity with status 500: got nil error")
	}
	var rateErr *RateLimitError
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnauthorized) || errors.As(err, &rateErr) {
		t.Errorf("Activity with status 500: error %v must not be a typed error", err)
	}
	testutil.AssertEqual(t, "API calls", 1, fake.apiCount())
}
