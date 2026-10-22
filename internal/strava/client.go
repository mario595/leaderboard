package strava

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// DefaultBaseURL is the Strava API root.
const DefaultBaseURL = "https://www.strava.com/api/v3"

// API holds what all Strava clients share (OAuth config, base HTTP client,
// token store, API root) and builds one Client per user with ForUser.
type API struct {
	cfg     *oauth2.Config
	http    *http.Client // base client: timeout, transport
	tokens  TokenStore
	baseURL string
}

// Client calls the Strava API on behalf of one user. Its HTTP client adds the
// user's access token to every request and refreshes it when it expires.
type Client struct {
	http    *http.Client
	baseURL string
}

// TokenStore saves a user's refreshed tokens. It is defined here, where it is
// consumed; *store.UserStore satisfies it.
type TokenStore interface {
	UpdateTokens(ctx context.Context, userID int64, accessToken, refreshToken string, expiresAt time.Time) error
}

// New returns an API. base must have a timeout: every per-user client reuses
// its Timeout and Transport, for API calls and token refreshes alike. baseURL
// is the API root, normally DefaultBaseURL.
func New(cfg *oauth2.Config, base *http.Client, tokens TokenStore, baseURL string) *API {
	return &API{
		cfg:     cfg,
		http:    base,
		tokens:  tokens,
		baseURL: baseURL,
	}
}

// ForUser returns a Client for the user with the given internal ID, starting
// from tok. The client refreshes tok when it expires. It is meant for one
// unit of work (one webhook event, one backfill run) and then dropped.
func (a *API) ForUser(userID int64, tok *oauth2.Token) *Client {
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, a.http)
	src := &savingTokenSource{
		src:    a.cfg.TokenSource(ctx, tok),
		userID: userID,
		store:  a.tokens,
		last:   tok.AccessToken,
	}
	return &Client{
		http: &http.Client{
			Timeout:   a.http.Timeout,
			Transport: &oauth2.Transport{Source: src, Base: a.http.Transport},
		},
		baseURL: a.baseURL,
	}
}

// get sends a GET request to path (relative to the API root) and decodes the
// JSON response into out, which must be a pointer. Status 404, 401 and 429
// become ErrNotFound, ErrUnauthorized and *RateLimitError; any other status
// than 200 is a plain error. A failed token refresh returns an error wrapping
// *oauth2.RetrieveError, and also ErrUnauthorized when Strava rejected the
// refresh token.
func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("strava GET %s: %w", path, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		var retrieveErr *oauth2.RetrieveError
		if errors.As(err, &retrieveErr) && retrieveErr.Response != nil &&
			(retrieveErr.Response.StatusCode == http.StatusBadRequest || retrieveErr.Response.StatusCode == http.StatusUnauthorized) {
			return fmt.Errorf("strava GET %s: %w: %w", path, ErrUnauthorized, err)
		}
		return fmt.Errorf("strava GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return fmt.Errorf("strava GET %s: %w", path, ErrNotFound)
		case http.StatusUnauthorized:
			return fmt.Errorf("strava GET %s: %w", path, ErrUnauthorized)
		case http.StatusTooManyRequests:
			shortUsage, dailyUsage := parsePair(resp.Header.Get("X-RateLimit-Usage"))
			shortLimit, dailyLimit := parsePair(resp.Header.Get("X-RateLimit-Limit"))
			rateLimitErr := &RateLimitError{
				ShortUsage: shortUsage,
				DailyUsage: dailyUsage,
				ShortLimit: shortLimit,
				DailyLimit: dailyLimit,
			}
			return fmt.Errorf("strava GET %s: %w", path, rateLimitErr)
		default:
			return fmt.Errorf("strava GET %s: status %d", path, resp.StatusCode)
		}
	}

	err = json.NewDecoder(resp.Body).Decode(out)
	if err != nil {
		return fmt.Errorf("strava GET %s: %w", path, err)
	}

	return nil
}

// parsePair parses a Strava rate-limit header such as "201,1803" into its
// 15-minute and daily values. A missing or malformed part is 0, on purpose:
// a 429 must become a RateLimitError even when the numbers are unusable.
func parsePair(h string) (short, daily int) {
	s, d, _ := strings.Cut(h, ",")
	short, _ = strconv.Atoi(s)
	daily, _ = strconv.Atoi(d)
	return short, daily
}
