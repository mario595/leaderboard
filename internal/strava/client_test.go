package strava

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/time/rate"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

// The compile-time check lives in a test file so that production code in
// this package does not import store.
var _ TokenStore = (*store.UserStore)(nil)

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

// One limiter for the whole API: requests of different users share it, and a
// request that cannot get a turn before its context ends is never sent.
func TestGet_limiterIsSharedByAllClients(t *testing.T) {
	fake := newFakeStrava(t)
	// Two requests at once, then one per hour: the third must wait.
	limiter := rate.NewLimiter(rate.Every(time.Hour), 2)
	api := newTestAPIWithLimiter(fake, &fakeTokenStore{}, 5*time.Second, limiter)
	alice := api.ForUser(7, validToken())
	bob := api.ForUser(8, validToken())

	var athlete struct{}
	if err := alice.get(t.Context(), "/athlete", &athlete); err != nil {
		t.Fatalf("first request: %v", err)
	}
	if err := bob.get(t.Context(), "/athlete", &athlete); err != nil {
		t.Fatalf("second request (within the burst): %v", err)
	}

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := alice.get(ctx, "/athlete", &athlete)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("third request: got nil error, want it held back by the limiter")
	}
	testutil.AssertEqual(t, "requests that reached Strava", 2, fake.apiCount())
	// Wait gives up at once when the context ends before the next turn; it
	// does not sleep until the deadline.
	if elapsed > time.Second {
		t.Errorf("third request took %v, want it to give up within the context's 100ms", elapsed)
	}
}

// A request that gets a turn goes through: the limiter delays, it does not drop.
func TestGet_limiterDelaysUntilTheNextTurn(t *testing.T) {
	fake := newFakeStrava(t)
	limiter := rate.NewLimiter(rate.Every(200*time.Millisecond), 1)
	client := newTestAPIWithLimiter(fake, &fakeTokenStore{}, 5*time.Second, limiter).ForUser(7, validToken())

	var athlete struct{}
	start := time.Now()
	for i := range 3 {
		if err := client.get(t.Context(), "/athlete", &athlete); err != nil {
			t.Fatalf("request %d: %v", i+1, err)
		}
	}
	elapsed := time.Since(start)

	testutil.AssertEqual(t, "requests that reached Strava", 3, fake.apiCount())
	// The first is free, the next two wait 200ms each.
	if elapsed < 350*time.Millisecond {
		t.Errorf("3 requests at one per 200ms took %v, want at least ~400ms", elapsed)
	}
}

func TestSave_expiredTokenIsSavedOnce(t *testing.T) {
	fake := newFakeStrava(t)
	tokens := &fakeTokenStore{}
	client := newTestAPIWith(fake, tokens, 5*time.Second).ForUser(7, expiredToken())

	var athlete struct{}
	if err := client.get(t.Context(), "/athlete", &athlete); err != nil {
		t.Fatalf("first get: %v", err)
	}
	// The second request reuses the refreshed token: no new refresh, no new save.
	if err := client.get(t.Context(), "/athlete", &athlete); err != nil {
		t.Fatalf("second get: %v", err)
	}

	saves := tokens.savedCalls()
	if len(saves) != 1 {
		t.Fatalf("saves: got %d, wanted 1 (%+v)", len(saves), saves)
	}
	got := saves[0]
	testutil.AssertEqual(t, "userID", int64(7), got.userID)
	testutil.AssertEqual(t, "access token", "new-access", got.accessToken)
	// Strava rotates refresh tokens: the new one must be stored too.
	testutil.AssertEqual(t, "refresh token", "new-refresh", got.refreshToken)
	if want := time.Now().Add(6 * time.Hour); got.expiresAt.Before(want.Add(-time.Minute)) || got.expiresAt.After(want.Add(time.Minute)) {
		t.Errorf("expiresAt: got %v, wanted about %v", got.expiresAt, want)
	}
	testutil.AssertEqual(t, "refresh calls", 1, fake.refreshCount())
}

func TestSave_validTokenIsNotSaved(t *testing.T) {
	fake := newFakeStrava(t)
	tokens := &fakeTokenStore{}
	client := newTestAPIWith(fake, tokens, 5*time.Second).ForUser(7, validToken())

	var athlete struct{}
	if err := client.get(t.Context(), "/athlete", &athlete); err != nil {
		t.Fatalf("get: %v", err)
	}
	testutil.AssertEqual(t, "saves", 0, len(tokens.savedCalls()))
}

func TestSave_failureFailsTheRequestAndIsRetried(t *testing.T) {
	fake := newFakeStrava(t)
	errDB := errors.New("database is down")
	tokens := &fakeTokenStore{failures: 1, err: errDB}
	client := newTestAPIWith(fake, tokens, 5*time.Second).ForUser(7, expiredToken())

	// First request: the refresh works, the save fails, so the request fails
	// before reaching the API. Going on with a token we could not store would
	// lose the rotated refresh token.
	var athlete struct{}
	err := client.get(t.Context(), "/athlete", &athlete)
	if !errors.Is(err, errDB) {
		t.Fatalf("get with failing save: got error %v, wanted one wrapping %v", err, errDB)
	}
	testutil.AssertEqual(t, "API calls after failed save", 0, fake.apiCount())

	// Second request: the token is still the refreshed one (no new refresh),
	// and saving it is tried again.
	if err := client.get(t.Context(), "/athlete", &athlete); err != nil {
		t.Fatalf("get after failed save: %v", err)
	}
	testutil.AssertEqual(t, "save attempts", 2, tokens.attemptCount())
	testutil.AssertEqual(t, "successful saves", 1, len(tokens.savedCalls()))
	testutil.AssertEqual(t, "refresh calls", 1, fake.refreshCount())
}

func TestSave_concurrentRequestsRefreshAndSaveOnce(t *testing.T) {
	fake := newFakeStrava(t)
	tokens := &fakeTokenStore{}
	client := newTestAPIWith(fake, tokens, 5*time.Second).ForUser(7, expiredToken())

	const workers = 10
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var athlete struct{}
			errs <- client.get(t.Context(), "/athlete", &athlete)
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Errorf("concurrent get: %v", err)
		}
	}
	testutil.AssertEqual(t, "refresh calls", 1, fake.refreshCount())
	testutil.AssertEqual(t, "saves", 1, len(tokens.savedCalls()))
	testutil.AssertEqual(t, "API calls", workers, fake.apiCount())
}

// fakeTokenStore records saves. The first `failures` calls return err.
// Saves happen on request goroutines, so fields are guarded by mu.
type fakeTokenStore struct {
	mu       sync.Mutex
	failures int
	err      error
	attempts int
	saved    []savedTokens
}

type savedTokens struct {
	userID       int64
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

func (f *fakeTokenStore) UpdateTokens(_ context.Context, userID int64, accessToken, refreshToken string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attempts++
	if f.attempts <= f.failures {
		return f.err
	}
	f.saved = append(f.saved, savedTokens{userID, accessToken, refreshToken, expiresAt})
	return nil
}

func (f *fakeTokenStore) savedCalls() []savedTokens {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.saved)
}

func (f *fakeTokenStore) attemptCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts
}

// newTestAPI returns an API whose token endpoint and API root both point at
// the fake server, with a token store that accepts every save.
func newTestAPI(fake *fakeStrava) *API {
	return newTestAPIWith(fake, &fakeTokenStore{}, 5*time.Second)
}

func newTestAPIWithTimeout(fake *fakeStrava, timeout time.Duration) *API {
	return newTestAPIWith(fake, &fakeTokenStore{}, timeout)
}

func newTestAPIWith(fake *fakeStrava, tokens TokenStore, timeout time.Duration) *API {
	return newTestAPIWithLimiter(fake, tokens, timeout, nil)
}

// newTestAPIWithLimiter is newTestAPIWith plus a rate limiter (nil: none).
func newTestAPIWithLimiter(fake *fakeStrava, tokens TokenStore, timeout time.Duration, limiter *rate.Limiter) *API {
	cfg := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint: oauth2.Endpoint{
			TokenURL:  fake.srv.URL + "/oauth/token",
			AuthStyle: oauth2.AuthStyleInParams, // as Strava requires
		},
	}
	return New(cfg, &http.Client{Timeout: timeout}, tokens, fake.srv.URL, limiter)
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
	rateUsage    string        // set by tests: X-RateLimit-Usage on API responses
	refreshes    int
	grantType    string
	refreshToken string
	apiCalls     int
	apiAuth      string
	apiPath      string
	listBody     string     // set by tests: replaces the first page of activities
	listQuery    url.Values // the query of the last activity list request
}

func newFakeStrava(t *testing.T) *fakeStrava {
	t.Helper()
	f := &fakeStrava{apiStatus: http.StatusOK, tokenStatus: http.StatusOK, rateUsage: "31,512"}

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

	// Activity IDs this fake knows, mapped to fixtures in testdata.
	activities := map[string]string{
		"12345678901": "activity_run.json",
		"12345678902": "activity_race_ride.json",
	}
	mux.HandleFunc("GET /activities/{id}", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.apiCalls++
		f.apiAuth = r.Header.Get("Authorization")
		f.apiPath = r.URL.Path
		status, delay, usage := f.apiStatus, f.delay, f.rateUsage
		f.mu.Unlock()
		time.Sleep(delay)

		// Strava sends its rate limits on every API response.
		w.Header().Set("X-RateLimit-Limit", "200,2000")
		w.Header().Set("X-RateLimit-Usage", usage)

		if status != http.StatusOK {
			http.Error(w, `{"message":"error"}`, status)
			return
		}
		fixture, ok := activities[r.PathValue("id")]
		if !ok {
			http.Error(w, `{"message":"Record Not Found"}`, http.StatusNotFound)
			return
		}
		data, err := os.ReadFile("testdata/" + fixture)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
	})

	// The athlete's activity list: one page from a fixture (or listBody),
	// then empty pages.
	mux.HandleFunc("GET /athlete/activities", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.apiCalls++
		f.apiAuth = r.Header.Get("Authorization")
		f.apiPath = r.URL.Path
		f.listQuery = r.URL.Query()
		status, delay, usage, body := f.apiStatus, f.delay, f.rateUsage, f.listBody
		f.mu.Unlock()
		time.Sleep(delay)

		w.Header().Set("X-RateLimit-Limit", "200,2000")
		w.Header().Set("X-RateLimit-Usage", usage)

		if status != http.StatusOK {
			http.Error(w, `{"message":"error"}`, status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("page") != "1" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if body != "" {
			_, _ = w.Write([]byte(body))
			return
		}
		data, err := os.ReadFile("testdata/activities_page.json")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(data)
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

func (f *fakeStrava) lastAPIPath() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.apiPath
}

func (f *fakeStrava) apiCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.apiCalls
}

func (f *fakeStrava) lastListQuery() url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listQuery
}

func (f *fakeStrava) lastAPIAuth() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.apiAuth
}
