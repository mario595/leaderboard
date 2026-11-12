package strava

import "context"

// athleteDTO is the part of GET /athlete that CheckAccess reads: nothing but
// proof that the call worked.
type athleteDTO struct {
	ID int64 `json:"id"`
}

// CheckAccess asks Strava whether it still accepts the app's authorization
// for this user, with one GET /athlete (refreshing and saving the token first
// when it has expired). It returns nil when it does, an error wrapping
// ErrUnauthorized when Strava rejected the token or the refresh (the user
// revoked the app, or the app revoked itself when they deleted their account),
// and any other error when the answer is unknown (rate limit, network,
// Strava down).
func (c *Client) CheckAccess(ctx context.Context) error {
	var athlete athleteDTO
	return c.get(ctx, "/athlete", &athlete)
}
