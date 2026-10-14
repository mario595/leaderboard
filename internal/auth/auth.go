package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/mario595/leaderboard/internal/store"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

// UserStore is the part of the user storage that the login flow needs.
// It is defined here, where it is consumed, and *store.UserStore satisfies it.
type UserStore interface {
	// Upsert inserts the user, or updates it if its Strava ID already exists.
	Upsert(ctx context.Context, u store.User) (store.User, error)
}

var _ UserStore = (*store.UserStore)(nil)

// Handler serves the Strava OAuth login and callback endpoints.
type Handler struct {
	oauthCfg      *oauth2.Config
	userStore     UserStore
	logger        *slog.Logger
	client        *http.Client
	secureCookies bool
}

// New returns a Handler. The client is used for the token exchange with Strava
// and should have a timeout. Set secureCookies to true when serving over HTTPS.
func New(cfg *oauth2.Config, us UserStore, logger *slog.Logger, client *http.Client, secureCookies bool) *Handler {
	return &Handler{
		oauthCfg:      cfg,
		userStore:     us,
		logger:        logger,
		client:        client,
		secureCookies: secureCookies,
	}
}

// NewOauthConfig returns the OAuth configuration for Strava. Strava redirects
// back to baseURL + "/auth/strava/callback", and the app asks for the "read"
// and "activity:read" scopes.
func NewOauthConfig(clientID, clientSecret, baseURL string) *oauth2.Config {
	ep := endpoints.Strava
	ep.AuthStyle = oauth2.AuthStyleInParams
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  baseURL + "/auth/strava/callback",
		Endpoint:     ep,
		Scopes:       []string{"read", "activity:read"},
	}
}
