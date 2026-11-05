package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

// A known athlete logs in without an invitation. If they opened a link
// anyway, the code is dropped: it must not linger in the session.
func TestCallback_knownAthleteWithInvitation(t *testing.T) {
	fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
	users := &fakeUserStore{}
	h := newHandlerWith(fs, users)

	rec := callbackWithInvitation(t, h, "ABC234")

	testutil.AssertEqual(t, "Location", "/", rec.Result().Header.Get("Location"))
	testutil.AssertEqual(t, "Login calls", 1, users.loginCalls)
	testutil.AssertEqual(t, "SignUp calls", 0, users.signUpCalls)
	session := sessionCookieOf(rec)
	if session == nil {
		t.Fatal("no session cookie after login")
	}
	testutil.AssertEqual(t, "user in the session", int64(1), sessionUserID(t, h, session))
	testutil.AssertEqual(t, "code left in the session", "", sessionString(t, h, session, invitationKey))
}

func TestCallback_newAthleteSignsUp(t *testing.T) {
	fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
	users := &fakeUserStore{loginErr: store.ErrNotFound}
	h := newHandlerWith(fs, users)

	rec := callbackWithInvitation(t, h, "ABC234")

	testutil.AssertEqual(t, "status", http.StatusSeeOther, rec.Code)
	testutil.AssertEqual(t, "Location", "/", rec.Result().Header.Get("Location"))
	testutil.AssertEqual(t, "Login calls", 1, users.loginCalls)
	testutil.AssertEqual(t, "SignUp calls", 1, users.signUpCalls)
	testutil.AssertEqual(t, "invitation code", "ABC234", users.gotCode)
	testutil.AssertEqual(t, "StravaID", int64(4242), users.got.StravaID)
	testutil.AssertEqual(t, "FirstName", "Ada", users.got.FirstName)
	testutil.AssertEqual(t, "LastName", "Lovelace", users.got.LastName)
	testutil.AssertEqual(t, "AccessToken", "AT", users.got.AccessToken)
	testutil.AssertEqual(t, "RefreshToken", "RT", users.got.RefreshToken)

	session := sessionCookieOf(rec)
	if session == nil {
		t.Fatal("no session cookie after sign-up")
	}
	// The ID SignUp returned (7), not Login's (1).
	testutil.AssertEqual(t, "user in the session", int64(7), sessionUserID(t, h, session))
	testutil.AssertEqual(t, "code left in the session", "", sessionString(t, h, session, invitationKey))
}

// A new athlete without an invitation link gets no account.
func TestCallback_newAthleteWithoutInvitation(t *testing.T) {
	fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
	users := &fakeUserStore{loginErr: store.ErrNotFound}
	h := newHandlerWith(fs, users)
	rec := httptest.NewRecorder()

	serveCallback(h, rec, validCallbackRequest(t))

	testutil.AssertEqual(t, "status", http.StatusSeeOther, rec.Code)
	testutil.AssertEqual(t, "Location", "/login?error=invitation", rec.Result().Header.Get("Location"))
	testutil.AssertEqual(t, "SignUp calls", 0, users.signUpCalls)
	if c := sessionCookieOf(rec); c != nil {
		t.Errorf("rejected sign-up sets a session cookie %q, want none", c.Name)
	}
}

// The store decides whether a code is usable. A rejected code is dropped from
// the session, so the next attempt does not reuse it.
func TestCallback_newAthleteSignUpFails(t *testing.T) {
	tests := []struct {
		name         string
		signUpErr    error
		wantLocation string
	}{
		{
			name:         "invalid invitation",
			signUpErr:    fmt.Errorf("sign up athlete 4242: %w", store.ErrInvalidInvitation),
			wantLocation: "/login?error=invitation",
		},
		{
			// Not the invitation's fault: trying again may work.
			name:         "store failure",
			signUpErr:    errors.New("database is locked"),
			wantLocation: "/login?error=failed",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
			users := &fakeUserStore{loginErr: store.ErrNotFound, signUpErr: tc.signUpErr}
			h := newHandlerWith(fs, users)
			invite := sessionCookieOf(serveInvite(t, h, "ABC234"))

			req := validCallbackRequest(t)
			req.AddCookie(invite)
			rec := httptest.NewRecorder()
			serveCallback(h, rec, req)

			testutil.AssertEqual(t, "status", http.StatusSeeOther, rec.Code)
			testutil.AssertEqual(t, "Location", tc.wantLocation, rec.Result().Header.Get("Location"))
			testutil.AssertEqual(t, "SignUp calls", 1, users.signUpCalls)

			// The session still exists (it held the code), but nobody is logged in.
			session := sessionCookieOf(rec)
			if session == nil {
				session = invite
			}
			testutil.AssertEqual(t, "user in the session", int64(0), sessionUserID(t, h, session))
			testutil.AssertEqual(t, "code left in the session", "", sessionString(t, h, session, invitationKey))
		})
	}
}

// callbackWithInvitation opens the invitation link, then comes back from
// Strava with the same session, as the browser does.
func callbackWithInvitation(t *testing.T, h *Handler, code string) *httptest.ResponseRecorder {
	t.Helper()
	invite := sessionCookieOf(serveInvite(t, h, code))
	if invite == nil {
		t.Fatal("setup: the invitation link set no session cookie")
	}
	req := validCallbackRequest(t)
	req.AddCookie(invite)
	rec := httptest.NewRecorder()
	serveCallback(h, rec, req)
	return rec
}

// A sign-up tells the listener (the backfill), with the new user.
func TestCallback_signUpNotifiesTheListener(t *testing.T) {
	fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
	listener := &fakeListener{}
	h := newHandlerWithListener(fs, &fakeUserStore{loginErr: store.ErrNotFound}, listener)

	rec := callbackWithInvitation(t, h, "ABC234")

	testutil.AssertEqual(t, "Location", "/", rec.Result().Header.Get("Location"))
	if len(listener.got) != 1 {
		t.Fatalf("listener calls = %d, want 1", len(listener.got))
	}
	testutil.AssertEqual(t, "user ID", int64(7), listener.got[0].ID)
	testutil.AssertEqual(t, "StravaID", int64(4242), listener.got[0].StravaID)
}

// A failing listener does not fail the login: the account exists, and the
// backfill can be queued again from the CLI.
func TestCallback_signUpListenerFails(t *testing.T) {
	fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
	listener := &fakeListener{err: errors.New("database is locked")}
	h := newHandlerWithListener(fs, &fakeUserStore{loginErr: store.ErrNotFound}, listener)

	rec := callbackWithInvitation(t, h, "ABC234")

	testutil.AssertEqual(t, "Location", "/", rec.Result().Header.Get("Location"))
	session := sessionCookieOf(rec)
	if session == nil {
		t.Fatal("no session cookie: the sign-up must still log the user in")
	}
	testutil.AssertEqual(t, "user in the session", int64(7), sessionUserID(t, h, session))
	testutil.AssertEqual(t, "listener calls", 1, len(listener.got))
}

// Only a sign-up is news: not a known athlete's login, not a rejected
// sign-up.
func TestCallback_listenerOnlyOnSignUp(t *testing.T) {
	tests := []struct {
		name  string
		users *fakeUserStore
	}{
		{"known athlete logs in", &fakeUserStore{}},
		{"invalid invitation", &fakeUserStore{loginErr: store.ErrNotFound, signUpErr: store.ErrInvalidInvitation}},
		{"store failure", &fakeUserStore{loginErr: store.ErrNotFound, signUpErr: errors.New("database is locked")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
			listener := &fakeListener{}
			h := newHandlerWithListener(fs, tc.users, listener)

			callbackWithInvitation(t, h, "ABC234")

			testutil.AssertEqual(t, "listener calls", 0, len(listener.got))
		})
	}
}

// A handler without a listener (nil) signs up as before.
func TestCallback_signUpWithoutListener(t *testing.T) {
	fs := newFakeStrava(t, http.StatusOK, validTokenJSON)
	h := newHandlerWithListener(fs, &fakeUserStore{loginErr: store.ErrNotFound}, nil)

	rec := callbackWithInvitation(t, h, "ABC234")

	testutil.AssertEqual(t, "Location", "/", rec.Result().Header.Get("Location"))
}

type fakeListener struct {
	got []store.User
	err error
}

func (f *fakeListener) SignedUp(_ context.Context, u store.User) error {
	f.got = append(f.got, u)
	return f.err
}
