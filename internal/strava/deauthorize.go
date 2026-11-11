package strava

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// DefaultDeauthorizeURL is Strava's endpoint for revoking an app's access to
// an athlete. It is an OAuth endpoint, outside the API root.
const DefaultDeauthorizeURL = "https://www.strava.com/oauth/deauthorize"

// Deauthorize revokes the app's access to the user on Strava: every token
// the app holds for them stops working, and the app disappears from their
// Strava "My Apps" page. Strava then sends an athlete deauthorization event
// to the webhook.
//
// It waits for the limiter like any other call, then sends the access token
// as a form value, refreshing (and saving) it first when it has expired.
// Errors map as in get: a 401, or a refresh that Strava rejected, wraps
// ErrUnauthorized (access already revoked); a 429 is a *RateLimitError.
func (c *Client) Deauthorize(ctx context.Context) error {
	const op = "strava deauthorize"
	if c.limiter != nil {
		if err := c.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("%s: wait for rate limiter: %w", op, err)
		}
	}
	tok, err := c.src.Token()
	if err != nil {
		return requestError(op, err)
	}

	form := url.Values{"access_token": {tok.AccessToken}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.deauthorizeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return requestError(op, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return statusError(op, resp)
	}
	return nil
}
