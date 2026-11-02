package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// invitationKey is the session key under which Invite keeps the invitation
// code until Callback uses it.
const invitationKey = "invitation"

// Invite handles an invitation link, GET /invite/{code}: it keeps the code in
// the session and starts the Strava login. It does not check the code: the
// callback is the single place that decides, when it knows whether the
// athlete is new.
func (h *Handler) Invite(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	h.sessions.Put(r.Context(), invitationKey, code)
	http.Redirect(w, r, "/auth/strava", http.StatusSeeOther)
}
