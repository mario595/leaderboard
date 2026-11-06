package worker

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
	"github.com/mario595/leaderboard/internal/testutil"
)

func TestBackoff(t *testing.T) {
	testCases := []struct {
		attempts int // failed attempts before this one
		want     time.Duration
	}{
		{0, time.Minute},
		{1, 2 * time.Minute},
		{2, 4 * time.Minute},
		{5, 32 * time.Minute},
		{6, time.Hour}, // 64 minutes, capped
		{7, time.Hour},
		// A naive 1m << attempts overflows int64 here and turns negative.
		{40, time.Hour},
	}
	for _, tc := range testCases {
		testutil.AssertEqual(t, "backoff", tc.want, backoff(tc.attempts))
	}
}

func TestHandle(t *testing.T) {
	now := time.Date(2026, 10, 15, 12, 7, 30, 0, time.UTC)
	ride := strava.Activity{ID: activityID, SportType: "Ride", StartLocal: now, MovingTime: time.Hour}
	rateLimited := &strava.RateLimitError{ShortUsage: 201, ShortLimit: 200, DailyUsage: 900, DailyLimit: 2000}

	testCases := []struct {
		name        string
		event       store.Event
		fetch       fetchResult
		wantDone    bool
		wantFailed  bool
		wantRetryAt time.Time // zero: no retry
		wantErrText string    // in last_error, for retries and failures
	}{
		{name: "success: done", event: activityEvent("create"), fetch: fetchResult{a: ride}, wantDone: true},
		{name: "ignored event: done", event: store.Event{ID: 1, ObjectType: "athlete", OwnerID: athleteID}, wantDone: true},
		{
			// The attempt limit applies to failures only: a success on the
			// last allowed attempt is done.
			name: "success on the eighth attempt: done", event: withAttempts(activityEvent("create"), 7),
			fetch: fetchResult{a: ride}, wantDone: true,
		},
		{
			name: "unauthorized: failed at once", event: activityEvent("create"),
			fetch: fetchResult{err: strava.ErrUnauthorized}, wantFailed: true, wantErrText: "unauthorized",
		},
		{
			name: "rate limited: retry at the next window", event: activityEvent("create"),
			fetch:       fetchResult{err: rateLimited},
			wantRetryAt: time.Date(2026, 10, 15, 12, 15, 0, 0, time.UTC), wantErrText: "rate limited",
		},
		{
			name: "first failure: retry in a minute", event: activityEvent("create"),
			fetch:       fetchResult{err: errors.New("status 500")},
			wantRetryAt: now.Add(time.Minute), wantErrText: "status 500",
		},
		{
			name: "fourth failure: retry in eight minutes", event: withAttempts(activityEvent("create"), 3),
			fetch:       fetchResult{err: errors.New("status 500")},
			wantRetryAt: now.Add(8 * time.Minute), wantErrText: "status 500",
		},
		{
			// maxAttempts = 8: the 8th failed attempt is the last.
			name: "eighth failure: failed", event: withAttempts(activityEvent("create"), 7),
			fetch:      fetchResult{err: errors.New("status 500")},
			wantFailed: true, wantErrText: "status 500",
		},
		{
			// Rate limits are not the event's fault: no attempt limit.
			name: "rate limited after many attempts: still retried", event: withAttempts(activityEvent("create"), 7),
			fetch:       fetchResult{err: rateLimited},
			wantRetryAt: time.Date(2026, 10, 15, 12, 15, 0, 0, time.UTC), wantErrText: "rate limited",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, []fetchResult{tc.fetch})
			f.worker.now = func() time.Time { return now }
			id := tc.event.ID

			f.worker.handle(t.Context(), tc.event)

			q := f.events
			q.mu.Lock()
			defer q.mu.Unlock()
			testutil.AssertEqual(t, "done", tc.wantDone, len(q.done) == 1 && q.done[0] == id)
			_, failed := q.failed[id]
			testutil.AssertEqual(t, "failed", tc.wantFailed, failed)
			retryAt, retried := q.retried[id]
			testutil.AssertEqual(t, "retried", !tc.wantRetryAt.IsZero(), retried)
			if retried && !retryAt.Equal(tc.wantRetryAt) {
				t.Errorf("retry at: got %v, wanted %v", retryAt, tc.wantRetryAt)
			}
			if marks := len(q.done) + len(q.failed) + len(q.retried); marks != 1 {
				t.Errorf("event marked %d times, wanted exactly once", marks)
			}
			if tc.wantErrText != "" {
				msg := q.failed[id] + q.errMsgs[id]
				if !strings.Contains(msg, tc.wantErrText) {
					t.Errorf("last_error %q does not contain %q", msg, tc.wantErrText)
				}
			}
		})
	}
}

func withAttempts(e store.Event, n int) store.Event {
	e.Attempts = n
	return e
}
