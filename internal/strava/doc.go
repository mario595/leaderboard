// Package strava calls the Strava API on behalf of users.
//
// An API is created once with the app's OAuth config, a base HTTP client
// (which must have a timeout) and a TokenStore. ForUser returns a Client for
// one user. The Client adds the user's access token to every request; when
// the token expires it refreshes it with the refresh token, saves the new
// tokens through the TokenStore (Strava can rotate the refresh token), and
// only then sends the request. Concurrent requests on one Client trigger a
// single refresh and a single save.
//
// Nothing calls this package yet: the webhook worker (#14) and the backfill
// (#22) will.
package strava
