package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/alexedwards/scs/v2"
	"github.com/mario595/leaderboard/internal/auth"
	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

type fakeUserStore struct {
	user  store.User
	err   error
	asked []int64
}

func (f *fakeUserStore) GetByID(_ context.Context, id int64) (store.User, error) {
	f.asked = append(f.asked, id)
	if f.err != nil {
		return store.User{}, f.err
	}
	return f.user, nil
}

func TestMeHandler_happyPath(t *testing.T) {
	users := &fakeUserStore{
		user: store.User{
			ID:           42,
			StravaID:     4242,
			FirstName:    "Ada",
			LastName:     "Lovelace",
			AccessToken:  "accessToken",
			RefreshToken: "refreshToken",
		},
	}

	meApp := newMeApp(t, users)
	_, _, _, set := meApp.do(t, "/login")
	status, body, _, _ := meApp.do(t, "/api/me", sessionCookie(t, set))

	testutil.AssertEqual(t, "status", http.StatusOK, status)
	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", body, err)
	}

	testutil.AssertEqual[any](t, "id", float64(42), got["id"])
	testutil.AssertEqual[any](t, "stravaId", float64(4242), got["stravaId"])
	testutil.AssertEqual[any](t, "firstName", "Ada", got["firstName"])
	testutil.AssertEqual[any](t, "lastName", "Lovelace", got["lastName"])

	for _, secret := range []string{"accessToken", "refreshToken"} {
		if strings.Contains(body, secret) {
			t.Errorf("body %q leaks %q", body, secret)
		}
	}
	if len(users.asked) != 1 {
		t.Fatalf("userGetter not called 1 time but %d", len(users.asked))
	}
	if users.asked[0] != 42 {
		t.Errorf("Incorrect arg in call to userGetter, got %d, wanted 42", users.asked[0])
	}
}

func TestMeHandler_noSession(t *testing.T) {
	users := &fakeUserStore{
		user: store.User{
			ID:           42,
			StravaID:     4242,
			FirstName:    "Ada",
			LastName:     "Lovelace",
			AccessToken:  "accessToken",
			RefreshToken: "refreshToken",
		},
	}

	meApp := newMeApp(t, users)
	status, body, _, _ := meApp.do(t, "/api/me")

	testutil.AssertEqual(t, "status", http.StatusUnauthorized, status)
	if !strings.Contains(body, "unauthorized") {
		t.Errorf("incorrect error msg, got %q, expected 'unauthorized'", body)
	}
}

func TestMeHandler_deletedUser(t *testing.T) {
	users := &fakeUserStore{
		err: store.ErrNotFound,
	}

	meApp := newMeApp(t, users)
	_, _, _, set := meApp.do(t, "/login")
	status, body, _, _ := meApp.do(t, "/api/me", sessionCookie(t, set))

	testutil.AssertEqual(t, "status", http.StatusUnauthorized, status)
	if !strings.Contains(body, "unauthorized") {
		t.Errorf("incorrect error msg, got %q, expected 'unauthorized'", body)
	}
}

// TestMeHandler_deletedUserDestroysSession checks that a session whose user no
// longer exists is destroyed, so it cannot be used again.
func TestMeHandler_deletedUserDestroysSession(t *testing.T) {
	users := &fakeUserStore{err: store.ErrNotFound}
	meApp := newMeApp(t, users)
	_, _, _, set := meApp.do(t, "/login")
	cookie := sessionCookie(t, set)

	first, _, _, _ := meApp.do(t, "/api/me", cookie)
	second, _, _, _ := meApp.do(t, "/api/me", cookie)

	testutil.AssertEqual(t, "first status", http.StatusUnauthorized, first)
	testutil.AssertEqual(t, "second status", http.StatusUnauthorized, second)
	// The second request is stopped by requireAuth (there is no user in the
	// session any more), so the store was asked only once.
	testutil.AssertEqual(t, "times the store was asked", 1, len(users.asked))
}

// TestMeHandler_storeFailure checks that an unexpected store error becomes a
// generic 500 and that the internal error text does not reach the client.
func TestMeHandler_storeFailure(t *testing.T) {
	users := &fakeUserStore{err: errors.New("database is locked")}
	meApp := newMeApp(t, users)
	_, _, _, set := meApp.do(t, "/login")

	status, body, header, _ := meApp.do(t, "/api/me", sessionCookie(t, set))

	testutil.AssertEqual(t, "status", http.StatusInternalServerError, status)
	testutil.AssertEqual(t, "Content-Type", "application/json", header.Get("Content-Type"))
	if strings.Contains(body, "database is locked") {
		t.Errorf("body %q leaks the internal error", body)
	}

	var got map[string]string
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", body, err)
	}
	if got["error"] == "" {
		t.Errorf("body %q has no error message", body)
	}
}

// TestMeHandler_onlyPublicFields checks the exact set of fields of the response.
// The store user also holds tokens, and none of them may be exposed.
func TestMeHandler_onlyPublicFields(t *testing.T) {
	users := &fakeUserStore{user: store.User{
		ID: 42, StravaID: 4242, FirstName: "Ada", LastName: "Lovelace",
		AccessToken: "accessToken", RefreshToken: "refreshToken",
	}}
	meApp := newMeApp(t, users)
	_, _, _, set := meApp.do(t, "/login")

	status, body, header, _ := meApp.do(t, "/api/me", sessionCookie(t, set))

	testutil.AssertEqual(t, "status", http.StatusOK, status)
	testutil.AssertEqual(t, "Content-Type", "application/json", header.Get("Content-Type"))

	var got map[string]any
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("body %q is not JSON: %v", body, err)
	}
	for _, key := range []string{"id", "stravaId", "firstName", "lastName"} {
		if _, ok := got[key]; !ok {
			t.Errorf("body %q has no field %q", body, key)
		}
	}
	testutil.AssertEqual(t, "number of fields", 4, len(got))
}

func newMeApp(t *testing.T, users userGetter) *authApp {
	t.Helper()

	logger := slog.New(slog.DiscardHandler)
	sessions := scs.New()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login", func(w http.ResponseWriter, r *http.Request) {
		sessions.Put(r.Context(), auth.SessionUserIDKey, int64(42))
	})
	mux.Handle("GET /api/me", requireAuth(sessions, logger)(meHandler(users, sessions, logger)))
	return &authApp{handler: sessions.LoadAndSave(mux)}
}
