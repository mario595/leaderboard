package strava

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/mario595/leaderboard/internal/testutil"
)

// The real endpoint, as a literal: a wrong constant must fail here.
func TestDefaultDeauthorizeURL(t *testing.T) {
	testutil.AssertEqual(t, "DefaultDeauthorizeURL", "https://www.strava.com/oauth/deauthorize", DefaultDeauthorizeURL)
}

func TestNew_usesTheRealDeauthorizeURL(t *testing.T) {
	api := New(nil, &http.Client{}, nil, DefaultBaseURL, nil)

	testutil.AssertEqual(t, "deauthorizeURL", "https://www.strava.com/oauth/deauthorize", api.deauthorizeURL)
}

func TestDeauthorize_sendsTheAccessTokenAsAFormValue(t *testing.T) {
	fake := newFakeStrava(t)
	client := newTestAPI(fake).ForUser(7, validToken())

	if err := client.Deauthorize(t.Context()); err != nil {
		t.Fatalf("Deauthorize: %v", err)
	}

	calls, token, contentType := fake.deauthorized()
	testutil.AssertEqual(t, "deauthorize calls", 1, calls)
	testutil.AssertEqual(t, "access_token", "old-access", token)
	testutil.AssertEqual(t, "Content-Type", "application/x-www-form-urlencoded", contentType)
	testutil.AssertEqual(t, "refresh calls", 0, fake.refreshCount())
}

// An expired token is refreshed first: Strava would reject the stale one,
// and the new one must be saved like on any other call.
func TestDeauthorize_expiredTokenIsRefreshedFirst(t *testing.T) {
	fake := newFakeStrava(t)
	tokens := &fakeTokenStore{}
	client := newTestAPIWith(fake, tokens, 5*time.Second).ForUser(7, expiredToken())

	if err := client.Deauthorize(t.Context()); err != nil {
		t.Fatalf("Deauthorize: %v", err)
	}

	_, token, _ := fake.deauthorized()
	testutil.AssertEqual(t, "refresh calls", 1, fake.refreshCount())
	testutil.AssertEqual(t, "access_token", "new-access", token)
	tokens.mu.Lock()
	defer tokens.mu.Unlock()
	if len(tokens.saved) != 1 {
		t.Fatalf("saved tokens: want 1, got %d", len(tokens.saved))
	}
	testutil.AssertEqual(t, "saved access token", "new-access", tokens.saved[0].accessToken)
	testutil.AssertEqual(t, "saved for user", int64(7), tokens.saved[0].userID)
}

func TestDeauthorize_unauthorized(t *testing.T) {
	fake := newFakeStrava(t)
	fake.deauthStatus = http.StatusUnauthorized // already revoked on Strava
	client := newTestAPI(fake).ForUser(7, validToken())

	err := client.Deauthorize(t.Context())

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Deauthorize with status 401: want ErrUnauthorized, got %v", err)
	}
}

func TestDeauthorize_refreshRejected(t *testing.T) {
	fake := newFakeStrava(t)
	fake.tokenStatus = http.StatusBadRequest // the refresh token was revoked
	client := newTestAPI(fake).ForUser(7, expiredToken())

	err := client.Deauthorize(t.Context())

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("Deauthorize with a rejected refresh: want ErrUnauthorized, got %v", err)
	}
	calls, _, _ := fake.deauthorized()
	testutil.AssertEqual(t, "deauthorize calls", 0, calls)
}

// A 500 from the token endpoint is transient, not "unauthorized" (#11).
func TestDeauthorize_refreshServerErrorIsNotUnauthorized(t *testing.T) {
	fake := newFakeStrava(t)
	fake.tokenStatus = http.StatusInternalServerError
	client := newTestAPI(fake).ForUser(7, expiredToken())

	err := client.Deauthorize(t.Context())

	if err == nil {
		t.Fatal("Deauthorize with a failed refresh: got nil error")
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Errorf("a 500 on refresh is transient, got ErrUnauthorized: %v", err)
	}
}

func TestDeauthorize_rateLimited(t *testing.T) {
	fake := newFakeStrava(t)
	fake.deauthStatus = http.StatusTooManyRequests
	fake.rateUsage = "201,1803"
	client := newTestAPI(fake).ForUser(7, validToken())

	err := client.Deauthorize(t.Context())

	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("Deauthorize with status 429: want *RateLimitError, got %v", err)
	}
	testutil.AssertEqual(t, "short usage", 201, rl.ShortUsage)
}

func TestDeauthorize_errorStatus(t *testing.T) {
	fake := newFakeStrava(t)
	fake.deauthStatus = http.StatusInternalServerError
	client := newTestAPI(fake).ForUser(7, validToken())

	err := client.Deauthorize(t.Context())

	if err == nil {
		t.Fatal("Deauthorize with status 500: got nil error")
	}
	if errors.Is(err, ErrUnauthorized) {
		t.Errorf("status 500 is not unauthorized: %v", err)
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error %q does not mention the status", err)
	}
}

// The revoke shares the app-wide request budget, like every other call.
func TestDeauthorize_waitsForTheLimiter(t *testing.T) {
	fake := newFakeStrava(t)
	limiter := rate.NewLimiter(rate.Every(time.Hour), 1)
	limiter.Allow() // spend the only token
	client := newTestAPIWithLimiter(fake, &fakeTokenStore{}, 5*time.Second, limiter).ForUser(7, validToken())
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	err := client.Deauthorize(ctx)

	if err == nil {
		t.Fatal("Deauthorize with an empty limiter: got nil error")
	}
	calls, _, _ := fake.deauthorized()
	testutil.AssertEqual(t, "deauthorize calls", 0, calls)
}
