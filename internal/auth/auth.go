package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"github.com/mario595/leaderboard/internal/store"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"
)

// UserStore is the part of the user storage that the login flow needs.
// It is defined here, where it is consumed, and *store.UserStore satisfies it.
type UserStore interface {
	// Login updates a known athlete's name and tokens. It returns
	// store.ErrNotFound for a new athlete.
	Login(ctx context.Context, u store.User) (store.User, error)
	// SignUp creates a new athlete with an invitation code. It returns an
	// error wrapping store.ErrInvalidInvitation when the code cannot be used.
	SignUp(ctx context.Context, u store.User, code string) (store.User, error)
}

// SignUpListener is told about each new member, after a successful sign-up
// only (not a login). It runs inside the login request, so it must be quick
// and must not call Strava; an error is logged and the login goes on.
// *backfill.Runner satisfies it.
type SignUpListener interface {
	SignedUp(ctx context.Context, u store.User) error
}

var _ UserStore = (*store.UserStore)(nil)

// Handler serves the Strava OAuth login and callback endpoints.
type Handler struct {
	oauthCfg      *oauth2.Config
	userStore     UserStore
	logger        *slog.Logger
	client        *http.Client
	secureCookies bool
	sessions      *scs.SessionManager
	signUps       SignUpListener
}

// New returns a Handler. signUps is told about new members; nil means nobody.
// The client is used for the token exchange with Strava and should have a
// timeout. Set secureCookies to true when serving over HTTPS.
func New(cfg *oauth2.Config, us UserStore, signUps SignUpListener, sessions *scs.SessionManager, logger *slog.Logger, client *http.Client, secureCookies bool) *Handler {
	return &Handler{
		oauthCfg:      cfg,
		userStore:     us,
		logger:        logger,
		client:        client,
		secureCookies: secureCookies,
		sessions:      sessions,
		signUps:       signUps,
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
