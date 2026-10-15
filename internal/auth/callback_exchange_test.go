package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

type fakeStrava struct {
	server *httptest.Server
	form   url.Values // the form fields of the last token
}

// newFakeStrava is a local stand-in for Strava's token endpoint.
func newFakeStrava(t *testing.T, status int, body string) *fakeStrava {
	t.Helper()
	fs := &fakeStrava{}
	fs.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		fs.form = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(fs.server.Close)
	return fs
}

// fakeUserStore records the user it receives
type fakeUserStore struct {
	got   store.User
	calls int
	err   error
}

func (f *fakeUserStore) Upsert(_ context.Context, u store.User) (store.User, error) {
	f.calls++
	f.got = u
	if f.err != nil {
		return store.User{}, f.err
	}
	u.ID = 1
	return u, nil
}

func newHandlerWith(fs *fakeStrava, users UserStore) *Handler {
	cfg := NewOauthConfig("test-id", "test-secret", "http://localhost:8080")
	cfg.Endpoint.TokenURL = fs.server.URL + "/oauth/token"
	return New(cfg, users, slog.New(slog.DiscardHandler), fs.server.Client(), false)
}

const validTokenJSON = `{"token_type":"Bearer","access_token":"AT","refresh_token":"RT",
"expires_in":21600,"athlete":{"id":4242,"firstname":"Ada","lastname":"Lovelace"}}`

// validCallbackRequest returns a callback request that passes the validation
// (matching state cookie, a code and the activity:read scope).
func validCallbackRequest(t *testing.T) *http.Request {
	t.Helper()

	q := url.Values{
		"state": {"abc"},
		"code":  {"x"},
		"scope": {"read,activity:read"},
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/strava/callback?"+q.Encode(), nil)
	req.AddCookie(&http.Cookie{Name: "oauth_state", Value: "abc"})
	return req
}

// TestCallback_exchangeFailures covers what the handler does when Strava or the
// user store misbehave. In every case the user must get a generic error, and
// nothing sensitive may reach the response.
func TestCallback_exchangeFailures(t *testing.T) {
	storeErr := errors.New("database is locked")

	testCases := []struct {
		name          string
		stravaStatus  int
		stravaBody    string
		storeErr      error
		wantStatus    int
		wantStoreCall int
	}{
		{
			name:         "strava rejects the code",
			stravaStatus: http.StatusBadRequest,
			stravaBody:   `{"message":"Bad Request","errors":[{"resource":"RequestToken","field":"code","code":"invalid"}]}`,
			wantStatus:   http.StatusBadGateway,
		},
		{
			name:         "strava answers with invalid JSON",
			stravaStatus: http.StatusOK,
			stravaBody:   `this is not json`,
			wantStatus:   http.StatusBadGateway,
		},
		{
			name:         "token response without access token",
			stravaStatus: http.StatusOK,
			stravaBody:   `{"token_type":"Bearer","refresh_token":"RT","expires_in":21600,"athlete":{"id":4242,"firstname":"Ada","lastname":"Lovelace"}}`,
			wantStatus:   http.StatusBadGateway,
		},
		{
			name:         "token response without athlete",
			stravaStatus: http.StatusOK,
			stravaBody:   `{"token_type":"Bearer","access_token":"AT","refresh_token":"RT","expires_in":21600}`,
			wantStatus:   http.StatusBadGateway,
		},
		{
			name:         "athlete with id zero",
			stravaStatus: http.StatusOK,
			stravaBody:   `{"token_type":"Bearer","access_token":"AT","refresh_token":"RT","expires_in":21600,"athlete":{"id":0,"firstname":"Ada","lastname":"Lovelace"}}`,
			wantStatus:   http.StatusBadGateway,
		},
		{
			name:          "user store fails",
			stravaStatus:  http.StatusOK,
			stravaBody:    validTokenJSON,
			storeErr:      storeErr,
			wantStatus:    http.StatusInternalServerError,
			wantStoreCall: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeStrava(t, tc.stravaStatus, tc.stravaBody)
			users := &fakeUserStore{err: tc.storeErr}
			h := newHandlerWith(fs, users)
			rec := httptest.NewRecorder()

			h.Callback(rec, validCallbackRequest(t))

			testutil.AssertEqual(t, "status", tc.wantStatus, rec.Code)
			testutil.AssertEqual(t, "store calls", tc.wantStoreCall, users.calls)

			// Nothing sensitive or internal may leak into the response.
			body := rec.Body.String()
			for _, secret := range []string{"test-secret", "AT", "RT", "database is locked"} {
				if strings.Contains(body, secret) {
					t.Errorf("response body %q leaks %q", body, secret)
				}
			}
		})
	}
}

// TestCallback_stravaTimeout checks that the handler gives up on a slow Strava
// instead of hanging, which proves that the bounded client is the one used for
// the token exchange.
func TestCallback_stravaTimeout(t *testing.T) {
	release := make(chan struct{})
	fs := &fakeStrava{}
	fs.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A Strava that never answers, until the test releases it.
		<-release
	}))
	// Cleanups run last-in, first-out: the handler is released first, so
	// closing the server does not wait for it.
	t.Cleanup(fs.server.Close)
	t.Cleanup(func() { close(release) })

	users := &fakeUserStore{}
	h := newHandlerWith(fs, users)
	h.client = &http.Client{Timeout: 50 * time.Millisecond}
	rec := httptest.NewRecorder()

	start := time.Now()
	h.Callback(rec, validCallbackRequest(t))
	elapsed := time.Since(start)

	testutil.AssertEqual(t, "status", http.StatusBadGateway, rec.Code)
	testutil.AssertEqual(t, "store calls", 0, users.calls)
	if elapsed > 2*time.Second {
		t.Errorf("handler took %v, want it to give up after the client timeout (50ms)", elapsed)
	}
}
