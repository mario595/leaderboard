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
	"golang.org/x/time/rate"
)

// DefaultBaseURL is the Strava API root.
const DefaultBaseURL = "https://www.strava.com/api/v3"

// API holds what all Strava clients share (OAuth config, base HTTP client,
// token store, API root, rate limiter) and builds one Client per user with
// ForUser.
type API struct {
	cfg            *oauth2.Config
	http           *http.Client // base client: timeout, transport
	tokens         TokenStore
	baseURL        string
	deauthorizeURL string        // DefaultDeauthorizeURL; tests point it at a fake
	limiter        *rate.Limiter // nil: no limit
}

// Client calls the Strava API on behalf of one user. Its HTTP client adds the
// user's access token to every request and refreshes it when it expires.
type Client struct {
	http           *http.Client
	src            oauth2.TokenSource // the user's tokens, refreshed and saved on demand
	baseURL        string
	deauthorizeURL string
	limiter        *rate.Limiter // the API's, shared with every other client
}

// TokenStore saves a user's refreshed tokens. It is defined here, where it is
// consumed; *store.UserStore satisfies it.
type TokenStore interface {
	UpdateTokens(ctx context.Context, userID int64, accessToken, refreshToken string, expiresAt time.Time) error
}

// New returns an API. base must have a timeout: every per-user client reuses
// its Timeout and Transport, for API calls and token refreshes alike. baseURL
// is the API root, normally DefaultBaseURL. limiter paces every request of
// every client built from this API; nil means no limit.
func New(cfg *oauth2.Config, base *http.Client, tokens TokenStore, baseURL string, limiter *rate.Limiter) *API {
	return &API{
		cfg:            cfg,
		http:           base,
		tokens:         tokens,
		baseURL:        baseURL,
		deauthorizeURL: DefaultDeauthorizeURL,
		limiter:        limiter,
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
		src:            src,
		baseURL:        a.baseURL,
		deauthorizeURL: a.deauthorizeURL,
		limiter:        a.limiter,
	}
}

// get sends a GET request to path (relative to the API root) and decodes the
// JSON response into out, which must be a pointer. It first waits for its turn
// with the limiter; if ctx ends before that, nothing is sent. Status 404, 401 and 429
// become ErrNotFound, ErrUnauthorized and *RateLimitError; any other status
// than 200 is a plain error. A failed token refresh returns an error wrapping
// *oauth2.RetrieveError, and also ErrUnauthorized when Strava rejected the
// refresh token.
func (c *Client) get(ctx context.Context, path string, out any) error {
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("strava GET %s: wait for rate limiter: %w", path, err)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("strava GET %s: %w", path, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return requestError("strava GET "+path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return statusError("strava GET "+path, resp)
	}

	err = json.NewDecoder(resp.Body).Decode(out)
	if err != nil {
		return fmt.Errorf("strava GET %s: %w", path, err)
	}

	return nil
}

// requestError wraps an error from sending a request (or from getting its
// token) under op. A refresh that Strava rejected (400 or 401 from the token
// endpoint) also wraps ErrUnauthorized, next to the *oauth2.RetrieveError;
// any other failure, such as a 500 from the token endpoint, is transient.
func requestError(op string, err error) error {
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && retrieveErr.Response != nil &&
		(retrieveErr.Response.StatusCode == http.StatusBadRequest || retrieveErr.Response.StatusCode == http.StatusUnauthorized) {
		return fmt.Errorf("%s: %w: %w", op, ErrUnauthorized, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// statusError turns a response whose status is not 200 into an error under
// op: 404, 401 and 429 become ErrNotFound, ErrUnauthorized and
// *RateLimitError (from the X-RateLimit headers); any other status is a plain
// error that names it.
func statusError(op string, resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %w", op, ErrUnauthorized)
	case http.StatusTooManyRequests:
		shortUsage, dailyUsage := parsePair(resp.Header.Get("X-RateLimit-Usage"))
		shortLimit, dailyLimit := parsePair(resp.Header.Get("X-RateLimit-Limit"))
		return fmt.Errorf("%s: %w", op, &RateLimitError{
			ShortUsage: shortUsage,
			DailyUsage: dailyUsage,
			ShortLimit: shortLimit,
			DailyLimit: dailyLimit,
		})
	default:
		return fmt.Errorf("%s: status %d", op, resp.StatusCode)
	}
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
