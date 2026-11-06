package backfill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"golang.org/x/oauth2"
)

// JobQueue is the backfill job queue; *store.BackfillStore satisfies it.
type JobQueue interface {
	Enqueue(ctx context.Context, userID, seasonID int64) (bool, error)
	NextDue(ctx context.Context) (store.BackfillJob, bool, error)
	MarkDone(ctx context.Context, id int64) error
	MarkRetry(ctx context.Context, id int64, errMsg string, notBefore time.Time) error
	MarkFailed(ctx context.Context, id int64, errMsg string) error
}

// Users loads the member a job is for; *store.UserStore satisfies it.
type Users interface {
	GetByID(ctx context.Context, id int64) (store.User, error)
}

// Seasons finds the season of a job, and the season covering a sign-up day;
// *store.SeasonStore satisfies it.
type Seasons interface {
	Current(ctx context.Context, date time.Time) (store.Season, error)
	Get(ctx context.Context, id int64) (store.Season, error)
}

// ClientFunc builds a Strava client acting for the user with the given
// internal ID. A function, not an interface, for the same reason as
// worker.ClientFunc: (*strava.API).ForUser returns *strava.Client.
type ClientFunc func(userID int64, tok *oauth2.Token) Client

// Runner imports seasons in the background, one job at a time, oldest
// first. SignedUp queues a job for a new member; the CLI queues jobs straight
// in the table, picked up at the next poll.
type Runner struct {
	jobs         JobQueue
	users        Users
	seasons      Seasons
	saver        ActivitySaver
	newClient    ClientFunc
	logger       *slog.Logger
	wake         chan struct{}
	now          func() time.Time
	pollInterval time.Duration
}

// NewRunner returns a Runner. It polls every minute for jobs that become due
// (retries, jobs queued by the CLI) and is woken by SignedUp.
func NewRunner(jobs JobQueue, users Users, seasons Seasons, saver ActivitySaver, newClient ClientFunc, logger *slog.Logger) *Runner {
	return &Runner{
		jobs:         jobs,
		users:        users,
		seasons:      seasons,
		newClient:    newClient,
		logger:       logger,
		saver:        saver,
		wake:         make(chan struct{}, 1),
		now:          time.Now,
		pollInterval: time.Minute,
	}
}

// Run processes due jobs until ctx is cancelled, then returns nil. It drains
// every due job, then waits for a wake, the next poll or cancellation.
// Cancellation also stops the job in progress at its next Strava request: a
// backfill can take half an hour, and since it is idempotent, the job just
// stays pending and runs again on the next start. A queue error is logged and
// retried at the next wake or poll.
func (r *Runner) Run(ctx context.Context) error {
	for {
		for {
			if ctx.Err() != nil {
				return nil
			}
			job, found, err := r.jobs.NextDue(ctx)
			if err != nil {
				r.logger.Error("next due backfill job", "error", err)
				break
			}
			if !found {
				break
			}
			r.handle(ctx, job)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-r.wake:
		case <-time.After(r.pollInterval):
		}
	}
}

// SignedUp queues a backfill of the season covering today (UTC) for a member
// who just signed up, and wakes the runner. Without a season today it does
// nothing. It never calls Strava, so the login stays fast; the auth package
// calls it after a sign-up.
func (r *Runner) SignedUp(ctx context.Context, u store.User) error {
	currentSeason, err := r.seasons.Current(ctx, r.now().UTC())
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("current season: %w", err)
	}
	_, err = r.jobs.Enqueue(ctx, u.ID, currentSeason.ID)
	if err != nil {
		return fmt.Errorf("enqueue for user %d, season %d: %w", u.ID, currentSeason.ID, err)
	}

	// Never block the login: when the buffer is full, a wake is already
	// pending, and the runner reads the whole table when it wakes.
	select {
	case r.wake <- struct{}{}:
	default:
	}
	r.logger.Info("backfill queued", "user_id", u.ID, "season_id", currentSeason.ID)

	return nil
}

// process imports the job's season for its user. A user or season deleted
// since the job was queued leaves nothing to do (nil). The returned error
// decides how handle marks the job.
func (r *Runner) process(ctx context.Context, job store.BackfillJob) error {
	user, err := r.users.GetByID(ctx, job.UserID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("retrieve user %d: %w", job.UserID, err)
	}

	season, err := r.seasons.Get(ctx, job.SeasonID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("retrieve season %d: %w", job.SeasonID, err)
	}

	client := r.newClient(user.ID, &oauth2.Token{
		AccessToken:  user.AccessToken,
		RefreshToken: user.RefreshToken,
		Expiry:       user.TokenExpiresAt,
	})

	r.logger.Info("backfill started", "job_id", job.ID, "user_id", user.ID, "season_id", season.ID,
		"from", season.StartDate.Format(time.DateOnly), "to", season.EndDate.Format(time.DateOnly),
		"attempt", job.Attempts+1)
	start := r.now()
	res, err := Import(ctx, client, r.saver, r.logger, user.ID, season.StartDate, season.EndDate)
	if err != nil {
		return fmt.Errorf("import for user %d and season %d (%d saved): %w", user.ID, season.ID, res.Saved, err)
	}

	r.logger.Info("backfill done", "job_id", job.ID, "user_id", user.ID, "season_id", season.ID,
		"activities", res.Saved, "details", res.Details, "duration", r.now().Sub(start))
	return nil
}
