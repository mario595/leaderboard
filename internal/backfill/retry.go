package backfill

import (
	"context"
	"errors"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
)

// maxAttempts is how many times a job may fail before it is marked failed.
// Rate limits do not count against it.
const maxAttempts = 5

// handle processes job and marks it: done on success; retried at the next
// rate limit window when rate limited; failed when Strava rejects the user's
// token or after maxAttempts failures; otherwise retried with backoff. A
// failure to mark is logged, and the job stays pending.
func (r *Runner) handle(ctx context.Context, job store.BackfillJob) {
	err := r.process(ctx, job)
	if err == nil {
		r.done(ctx, job.ID)
		return
	}
	var rl *strava.RateLimitError
	if errors.As(err, &rl) {
		r.retry(ctx, job.ID, err.Error(), rl.RetryAt(r.now()))
		return
	}
	if errors.Is(err, strava.ErrUnauthorized) || job.Attempts+1 >= maxAttempts {
		r.fail(ctx, job.ID, err.Error())
		return
	}

	r.retry(ctx, job.ID, err.Error(), r.now().Add(backoff(job.Attempts)))
}

// backoff is the wait before retrying a backfill job that has failed attempts
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

func (r *Runner) done(ctx context.Context, id int64) {
	if err := r.jobs.MarkDone(ctx, id); err != nil {
		r.logger.Error("mark backfill job done", "job_id", id, "error", err)
	}
}

func (r *Runner) retry(ctx context.Context, id int64, errMsg string, nextAttempt time.Time) {
	if err := r.jobs.MarkRetry(ctx, id, errMsg, nextAttempt); err != nil {
		r.logger.Error("mark backfill job for retry", "job_id", id, "error", err)
	}
}

func (r *Runner) fail(ctx context.Context, id int64, msg string) {
	if err := r.jobs.MarkFailed(ctx, id, msg); err != nil {
		r.logger.Error("mark backfill job failed", "job_id", id, "error", err)
	}
}
