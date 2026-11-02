package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/mario595/leaderboard/internal/store"
	"golang.org/x/oauth2"
)

type athlete struct {
	ID        int64  `json:"id"`
	FirstName string `json:"firstname"`
	LastName  string `json:"lastname"`
}

// redirectTo is where a successful login lands: the app's home page.
const redirectTo = "/"

// Reasons a failed login sends the user back to /login with, as ?error=. The
// login page only needs to know what the user can do about it; the precise
// cause goes to the log.
const (
	// loginCancelled: the user declined on Strava's consent screen.
	loginCancelled = "cancelled"
	// loginScope: the user unticked the permission to read activities.
	loginScope = "scope"
	// loginFailed: anything else (bad or missing state or code, Strava or
	// database failure). Trying again may work.
	loginFailed = "failed"
	// loginInvitation: a new athlete without a usable invitation (none,
	// unknown, used up, revoked or expired).
	loginInvitation = "invitation"
)

// SessionUserIDKey is the session key under which the id of the logged-in user
// is kept. Callback writes it, and the httpapi middleware reads it.
const SessionUserIDKey = "UserID"

// Callback handles the redirect from Strava after the user approved or
// declined the login. It rejects the request unless the state matches the
// cookie set by Login, the authorization code is present, and the user granted
// the activity:read (or activity:read_all) scope. The state cookie is
// cleared as soon as it has been read, so each state can be used only once.
// A known athlete is logged in. A new one is signed up with the invitation code
// that Invite left in the session, and turned away without one.
// Every outcome is a redirect back to the app: "/" when the user is logged in,
// "/login?error=<reason>" otherwise (see fail).
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("error") != "" {
		h.fail(w, r, loginCancelled, slog.LevelInfo, "login declined on Strava")
		return
	}
	c, err := r.Cookie("oauth_state")
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: "", Path: "/auth/strava", MaxAge: -1, HttpOnly: true, Secure: h.secureCookies})
	if err != nil {
		h.fail(w, r, loginFailed, slog.LevelWarn, "callback without state cookie")
		return
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		h.fail(w, r, loginFailed, slog.LevelWarn, "callback without state")
		return
	}

	if subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		h.fail(w, r, loginFailed, slog.LevelWarn, "callback state does not match the cookie")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		h.fail(w, r, loginFailed, slog.LevelWarn, "callback without code")
		return
	}

	scope := r.URL.Query().Get("scope")
	if scope == "" {
		h.fail(w, r, loginFailed, slog.LevelWarn, "callback without scope")
		return
	}
	scopeSlice := strings.FieldsFunc(scope, func(r rune) bool { return r == ',' || r == ' ' })

	if !slices.Contains(scopeSlice, "activity:read") && !slices.Contains(scopeSlice, "activity:read_all") {
		h.fail(w, r, loginScope, slog.LevelInfo, "activity:read scope not granted", "scope", scope)
		return
	}

	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, h.client)
	token, err := h.oauthCfg.Exchange(ctx, code)
	if err != nil {
		h.fail(w, r, loginFailed, slog.LevelError, "token exchange with Strava failed", "error", err)
		return
	}

	raw, err := json.Marshal(token.Extra("athlete"))
	if err != nil {
		h.fail(w, r, loginFailed, slog.LevelError, "encode athlete from token response", "error", err)
		return
	}
	var a athlete
	if err := json.Unmarshal(raw, &a); err != nil || a.ID == 0 {
		h.fail(w, r, loginFailed, slog.LevelError, "invalid athlete in token response", "error", err)
		return
	}

	// Popped on every path: a code is used for one login attempt at most, and
	// never lingers in the session of someone who already has an account.
	invitationCode := h.sessions.PopString(r.Context(), invitationKey)

	user := store.User{
		StravaID:       a.ID,
		FirstName:      a.FirstName,
		LastName:       a.LastName,
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
		TokenExpiresAt: token.Expiry,
	}

	u, err := h.userStore.Login(r.Context(), user)
	var knownAthlete bool
	switch {
	case err == nil:
		// A known athlete: logged in below.
		knownAthlete = true
	case errors.Is(err, store.ErrNotFound):
		// A new athlete: needs an invitation. The store decides whether the
		// code is usable.
		if invitationCode == "" {
			h.fail(w, r, loginInvitation, slog.LevelInfo, "invitation code required")
			return
		}
		u, err = h.userStore.SignUp(r.Context(), user, invitationCode)
		if err != nil {
			if errors.Is(err, store.ErrInvalidInvitation) {
				h.fail(w, r, loginInvitation, slog.LevelInfo, "sign up athlete: invalid invitation", "strava_id", a.ID)
				return
			}
			h.fail(w, r, loginFailed, slog.LevelError, "sign up athlete", "strava_id", a.ID, "error", err)
			return
		}
		h.logger.Info("user signed up", "strava_id", u.StravaID)
	default:
		h.fail(w, r, loginFailed, slog.LevelError, "login", "error", err)
		return
	}

	h.sessions.Put(r.Context(), SessionUserIDKey, u.ID)
	if err := h.sessions.RenewToken(r.Context()); err != nil {
		h.fail(w, r, loginFailed, slog.LevelError, "renew session token", "error", err)
		return
	}

	if knownAthlete {
		h.logger.Info("user logged in", "strava_id", u.StravaID)
	}

	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
}

// fail logs why a login did not complete and sends the user back to the login
// page with a reason the page can explain (one of the login* constants). The
// user never sees an error page. User choices (declining, unticking the
// permission) are logged at Info; broken requests at Warn; failures on
// Strava's or our side at Error.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, code string, level slog.Level, msg string, args ...any) {
	h.logger.Log(r.Context(), level, msg, args...)
	http.Redirect(w, r, "/login?error="+code, http.StatusSeeOther)
}
