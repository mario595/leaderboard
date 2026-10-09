package auth

import "net/http"

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	state := newState()
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/auth/strava",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, h.oauthCfg.AuthCodeURL(state), http.StatusFound)
}
