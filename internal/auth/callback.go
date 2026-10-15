package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
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

const redirectTo = "/api/me"

// SessionUserIDKey is the session key under which the id of the logged-in user
// is kept. Callback writes it, and the httpapi middleware reads it.
const SessionUserIDKey = "UserID"

// Callback handles the redirect from Strava after the user approved or
// declined the login. It rejects the request unless the state matches the
// cookie set by Login, the authorization code is present, and the user granted
// the activity:read (or activity:read_all) scope. The state cookie is
// cleared as soon as it has been read, so each state can be used only once.
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("error") != "" {
		http.Error(w, "login cancelled", http.StatusForbidden)
		return
	}
	c, err := r.Cookie("oauth_state")
	http.SetCookie(w, &http.Cookie{Name: "oauth_state", Value: "", Path: "/auth/strava", MaxAge: -1, HttpOnly: true, Secure: h.secureCookies})
	if err != nil {
		http.Error(w, "missing state", http.StatusBadRequest)
		return
	}
	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	if subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	scope := r.URL.Query().Get("scope")
	if scope == "" {
		http.Error(w, "missing scope", http.StatusBadRequest)
		return
	}
	scopeSlice := strings.FieldsFunc(scope, func(r rune) bool { return r == ',' || r == ' ' })

	if !slices.Contains(scopeSlice, "activity:read") && !slices.Contains(scopeSlice, "activity:read_all") {
		http.Error(w, "activity:read or activity:read_all scope is required", http.StatusForbidden)
		return
	}

	ctx := context.WithValue(r.Context(), oauth2.HTTPClient, h.client)
	token, err := h.oauthCfg.Exchange(ctx, code)
	if err != nil {
		h.logger.Error("token exchange error", "error", err)
		http.Error(w, "could not complete login with Strava", http.StatusBadGateway)
		return
	}

	raw, err := json.Marshal(token.Extra("athlete"))
	if err != nil {
		h.logger.Error("encode athlete", "error", err)
		http.Error(w, "unexpected response from Strava", http.StatusBadGateway)
		return
	}
	var a athlete
	if err := json.Unmarshal(raw, &a); err != nil || a.ID == 0 {
		h.logger.Error("invalid athlete in token response", "error", err)
		http.Error(w, "unexpected response from Strava", http.StatusBadGateway)
		return
	}

	u, err := h.userStore.Upsert(r.Context(), store.User{
		StravaID:       a.ID,
		FirstName:      a.FirstName,
		LastName:       a.LastName,
		AccessToken:    token.AccessToken,
		RefreshToken:   token.RefreshToken,
		TokenExpiresAt: token.Expiry,
	})
	if err != nil {
		h.logger.Error("upsert user failed", "error", err)
		http.Error(w, "could not save user", http.StatusInternalServerError)
		return
	}

	h.sessions.Put(r.Context(), SessionUserIDKey, u.ID)
	if err := h.sessions.RenewToken(r.Context()); err != nil {
		h.logger.Error("cannot login", "error", err)
		http.Error(w, "cannot login", http.StatusInternalServerError)
		return
	}

	h.logger.Info("user logged in", "strava_id", u.StravaID)

	http.Redirect(w, r, redirectTo, http.StatusSeeOther)
}
