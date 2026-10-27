package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/mario595/leaderboard/internal/auth"
	"github.com/mario595/leaderboard/internal/webhook"
)

type Deps struct {
	Logger     *slog.Logger
	Auth       *auth.Handler
	Sessions   *scs.SessionManager
	Users      userGetter
	Webhook    *webhook.Handler
	Seasons    seasonFinder
	Standings  standingsQuerier
	Activities activityLister
}

// NewRouter builds the http router with all the application routes.
func NewRouter(deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(requestLogger(deps.Logger))
	r.Use(middleware.Recoverer)
	r.Use(deps.Sessions.LoadAndSave)

	r.Get("/healthz", healthHandler(deps.Logger))
	if deps.Auth != nil {
		r.Get("/auth/strava", deps.Auth.Login)
		r.Get("/auth/strava/callback", deps.Auth.Callback)
		r.Post("/auth/logout", deps.Auth.Logout)
	}

	if deps.Users != nil {
		r.Route("/api", func(r chi.Router) {
			r.Use(requireAuth(deps.Sessions, deps.Users, deps.Logger))
			r.Get("/me", meHandler(deps.Logger))
			if deps.Seasons != nil && deps.Standings != nil {
				r.Get("/leaderboard", leaderboardHandler(deps.Seasons, deps.Standings, time.Now, deps.Logger))
			}
			if deps.Activities != nil {
				r.Get("/me/activities", activitiesHandler(deps.Seasons, deps.Activities, time.Now, deps.Logger))
			}
		})
	}

	if deps.Webhook != nil {
		r.Route("/webhook", func(r chi.Router) {
			r.Get("/strava", deps.Webhook.Verify)
			r.Post("/strava", deps.Webhook.Receive)
		})
	}

	return r
}
