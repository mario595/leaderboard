package strava

import (
	"errors"
	"fmt"
	"time"
)

// ErrNotFound means the activity does not exist or is private (Strava answers 404 for both).
var ErrNotFound = errors.New("strava: not found")

// ErrUnauthorized means the user revoked access and must log in again.
var ErrUnauthorized = errors.New("strava: unauthorized")

// RateLimitError means Strava answered 429: the app used up its 15-minute or
// daily request quota. Usage and limits come from the X-RateLimit-Usage and
// X-RateLimit-Limit headers; a value Strava did not send or that could not be
// parsed is 0. Windows reset at :00, :15, :30 and :45, and daily at midnight
// UTC. Match it with errors.As and a *RateLimitError target.
type RateLimitError struct {
	ShortUsage, ShortLimit, DailyUsage, DailyLimit int
}

// Error implements the error interface.
func (e *RateLimitError) Error() string {
	return fmt.Sprintf("strava: rate limited (15-minute usage %d/%d, daily %d/%d)", e.ShortUsage, e.ShortLimit, e.DailyUsage, e.DailyLimit)
}

// RetryAt returns when Strava's rate limit lets requests through again
// after e, seen at now: the next 15-minute boundary (UTC), or the next UTC
// midnight when the daily quota is used up. Zero limits mean the headers
// were missing: assume the 15-minute window.
func (e *RateLimitError) RetryAt(now time.Time) time.Time {
	t := now.UTC()
	nextQuarterHour := t.Truncate(15 * time.Minute).Add(15 * time.Minute)
	nextMidnight := time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, time.UTC)

	if e.DailyLimit > 0 && e.DailyUsage >= e.DailyLimit {
		return nextMidnight
	}
	return nextQuarterHour
}
