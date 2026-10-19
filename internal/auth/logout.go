package auth

import (
	"net/http"
)

// Logout ends the user's session and clears the session cookie. It does not fail when nobody is logged in.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Destroy(r.Context()); err != nil {
		h.logger.Error("logout error", "error", err)
		http.Error(w, "could not log out", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
