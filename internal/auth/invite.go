package auth

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// invitationKey is the session key under which Invite keeps the invitation
// code until Callback uses it.
const invitationKey = "invitation"

// joinPath is the frontend's consent page, shown before the Strava login of
// a new member (Strava API Policy 2.1 and 5.13). Its button starts the login.
const joinPath = "/join"

// Invite handles an invitation link, GET /invite/{code}: it keeps the code in
// the session and sends the browser to the consent page, which starts the
// Strava login once the athlete agrees. It does not check the code: the
// callback is the single place that decides, when it knows whether the
// athlete is new.
func (h *Handler) Invite(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	h.sessions.Put(r.Context(), invitationKey, code)
	http.Redirect(w, r, joinPath, http.StatusSeeOther)
}
