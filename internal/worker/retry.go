package worker

import (
	"context"
	"errors"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
)

// maxAttempts is how many times an event may fail before it is marked failed.
// Rate limits do not count against it.
const maxAttempts = 8

// backoff is the wait before retrying an event that has failed attempts
// times: one minute, doubled per attempt, capped at one hour.
func backoff(attempts int) time.Duration {
	result := time.Minute
	for range min(attempts, maxAttempts) {
		result *= 2
		if result >= time.Hour {
			return time.Hour
		}
	}

	return result
}

// handle processes e and marks it: done on success; retried at the next rate
// limit window when rate limited; failed when Strava rejects the user's
// token or after maxAttempts failures; otherwise retried with backoff. A
// failure to mark is logged, and the event stays pending.
func (w *Worker) handle(ctx context.Context, e store.Event) {
	err := w.process(ctx, e)

	if err == nil {
		w.done(ctx, e.ID)
		return
	}

	var rl *strava.RateLimitError
	if errors.As(err, &rl) {
		w.retry(ctx, e.ID, err.Error(), rl.RetryAt(w.now()))
		return
	}

	if e.Attempts+1 >= maxAttempts || errors.Is(err, strava.ErrUnauthorized) {
		w.fail(ctx, e.ID, err.Error())
		return
	}

	w.retry(ctx, e.ID, err.Error(), w.now().Add(backoff(e.Attempts)))
}

func (w *Worker) done(ctx context.Context, id int64) {
	if err := w.events.MarkDone(ctx, id); err != nil {
		w.logger.Error("mark event done", "event_id", id, "error", err)
	}
}

func (w *Worker) fail(ctx context.Context, id int64, errMsg string) {
	if err := w.events.MarkFailed(ctx, id, errMsg); err != nil {
		w.logger.Error("mark event failed", "event_id", id, "error", err)
	}
}

func (w *Worker) retry(ctx context.Context, id int64, errMsg string, nextAttempt time.Time) {
	if err := w.events.MarkRetry(ctx, id, errMsg, nextAttempt); err != nil {
		w.logger.Error("mark event for retry", "event_id", id, "error", err)
	}
}
