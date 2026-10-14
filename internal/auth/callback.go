package auth

import (
	"crypto/subtle"
	"net/http"
	"slices"
	"strings"
)

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

	if r.URL.Query().Get("code") == "" {
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

	w.WriteHeader(http.StatusNoContent)
}
