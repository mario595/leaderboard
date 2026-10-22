package strava

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestGet_validTokenSendsBearer(t *testing.T) {
	fake := newFakeStrava(t)
	client := newTestAPI(fake).ForUser(7, validToken())

	var athlete struct {
		ID        int64  `json:"id"`
		FirstName string `json:"firstname"`
	}
	if err := client.get(t.Context(), "/athlete", &athlete); err != nil {
		t.Fatalf("get: %v", err)
	}

	testutil.AssertEqual(t, "Authorization", "Bearer old-access", fake.lastAPIAuth())
	testutil.AssertEqual(t, "refresh calls", 0, fake.refreshCount())
	testutil.AssertEqual(t, "athlete ID", int64(42), athlete.ID)
	testutil.AssertEqual(t, "athlete first name", "Test", athlete.FirstName)
}

func TestGet_expiredTokenRefreshes(t *testing.T) {
	fake := newFakeStrava(t)
	client := newTestAPI(fake).ForUser(7, expiredToken())

	var athlete struct{}
	if err := client.get(t.Context(), "/athlete", &athlete); err != nil {
		t.Fatalf("get: %v", err)
	}

	testutil.AssertEqual(t, "refresh calls", 1, fake.refreshCount())
	testutil.AssertEqual(t, "grant_type", "refresh_token", fake.lastGrantType())
	testutil.AssertEqual(t, "refresh token sent", "old-refresh", fake.lastRefreshToken())
	testutil.AssertEqual(t, "Authorization", "Bearer new-access", fake.lastAPIAuth())
}

func TestGet_errorStatus(t *testing.T) {
	fake := newFakeStrava(t)
	fake.apiStatus = http.StatusInternalServerError
	client := newTestAPI(fake).ForUser(7, validToken())

	var athlete struct{}
	err := client.get(t.Context(), "/athlete", &athlete)
	if err == nil {
		t.Fatal("get with status 500: got nil error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("get with status 500: error %q does not mention the status", err)
	}
}

func TestGet_refreshFails(t *testing.T) {
	fake := newFakeStrava(t)
	fake.tokenStatus = http.StatusBadRequest // e.g. the refresh token was revoked
	client := newTestAPI(fake).ForUser(7, expiredToken())

	var athlete struct{}
	err := client.get(t.Context(), "/athlete", &athlete)

	// The oauth2 error must survive our wrapping: #12 will inspect it to tell
	// "user must log in again" apart from other failures.
	var retrieveErr *oauth2.RetrieveError
	if !errors.As(err, &retrieveErr) {
		t.Fatalf("get with failing refresh: got error %v, wanted a wrapped *oauth2.RetrieveError", err)
	}
	testutil.AssertEqual(t, "API calls", 0, fake.apiCount())
}

// The per-user client must keep the base client's timeout (oauth2.NewClient
// would silently drop it).
func TestGet_apiCallUsesBaseTimeout(t *testing.T) {
	fake := newFakeStrava(t)
	fake.delay = time.Second
	api := newTestAPIWithTimeout(fake, 100*time.Millisecond)

	var athlete struct{}
	start := time.Now()
	err := api.ForUser(7, validToken()).get(t.Context(), "/athlete", &athlete)
	if err == nil {
		t.Fatal("get against a slow API: got nil error, wanted a timeout")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("get against a slow API took %v, wanted about the 100ms timeout", elapsed)
	}
}

// The refresh must go through the base client too: oauth2 falls back to
// http.DefaultClient, which has no timeout, unless the TokenSource context
// carries our client.
func TestGet_refreshUsesBaseTimeout(t *testing.T) {
	fake := newFakeStrava(t)
	fake.delay = time.Second
	api := newTestAPIWithTimeout(fake, 100*time.Millisecond)

	// A request context without a deadline: only the client timeout can stop
	// the refresh.
	var athlete struct{}
	start := time.Now()
	err := api.ForUser(7, expiredToken()).get(t.Context(), "/athlete", &athlete)
	if err == nil {
		t.Fatal("get with a slow refresh: got nil error, wanted a timeout")
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("get with a slow refresh took %v, wanted about the 100ms timeout", elapsed)
	}
}

// newTestAPI returns an API whose token endpoint and API root both point at
// the fake server. Step 2 does not save tokens, so the TokenStore is nil.
func newTestAPI(fake *fakeStrava) *API {
	return newTestAPIWithTimeout(fake, 5*time.Second)
}

func newTestAPIWithTimeout(fake *fakeStrava, timeout time.Duration) *API {
	cfg := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL:  fake.srv.URL + "/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams, // as Strava requires
		},
	}
	return New(cfg, &http.Client{Timeout: timeout}, nil, fake.srv.URL)
}

func validToken() *oauth2.Token {
	return &oauth2.Token{
		AccessToken:  "old-access",
		RefreshToken: "old-refresh",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}
}

func expiredToken() *oauth2.Token {
	tok := validToken()
	tok.Expiry = time.Now().Add(-time.Minute)
	return tok
}

// fakeStrava serves the token endpoint and one API path, and records what it
// receives. Handlers run on server goroutines, so every field they touch is
// guarded by mu.
type fakeStrava struct {
	srv *httptest.Server

	mu           sync.Mutex
	apiStatus    int           // set by tests before the first request
	tokenStatus  int           // set by tests before the first request
	delay        time.Duration // set by tests: every handler sleeps this long
	refreshes    int
	grantType    string
	refreshToken string
	apiCalls     int
	apiAuth      string
}

func newFakeStrava(t *testing.T) *fakeStrava {
	t.Helper()
	f := &fakeStrava{apiStatus: http.StatusOK, tokenStatus: http.StatusOK}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f.mu.Lock()
		f.refreshes++
		f.grantType = r.PostForm.Get("grant_type")
		f.refreshToken = r.PostForm.Get("refresh_token")
		status, delay := f.tokenStatus, f.delay
		f.mu.Unlock()
		time.Sleep(delay)

		if status != http.StatusOK {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"message":"Bad Request","errors":[{"field":"refresh_token","code":"invalid"}]}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token_type":    "Bearer",
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    21600,
			"expires_at":    time.Now().Add(6 * time.Hour).Unix(),
		})
	})
	mux.HandleFunc("GET /athlete", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.apiCalls++
		f.apiAuth = r.Header.Get("Authorization")
		status, delay := f.apiStatus, f.delay
		f.mu.Unlock()
		time.Sleep(delay)

		if status != http.StatusOK {
			http.Error(w, `{"message":"error"}`, status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42,"firstname":"Test"}`))
	})

	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeStrava) refreshCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refreshes
}

func (f *fakeStrava) lastGrantType() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.grantType
}

func (f *fakeStrava) lastRefreshToken() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.refreshToken
}

func (f *fakeStrava) apiCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.apiCalls
}

func (f *fakeStrava) lastAPIAuth() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.apiAuth
}
