package auth

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mario595/leaderboard/internal/store"
)

type fakeStrava struct {
	server *httptest.Server
	form   url.Values // the form fields of the last token
}

// fakeStrava is a local stand-in for Strava's token endpoint.
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
