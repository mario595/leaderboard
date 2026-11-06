package reconcile

import (
	"context"
	"log/slog"
	"time"

	"github.com/mario595/leaderboard/internal/backfill"
	"github.com/mario595/leaderboard/internal/store"
	"golang.org/x/oauth2"
)

// Users lists every member; *store.UserStore satisfies it.
type Users interface {
	List(ctx context.Context) ([]store.User, error)
}

// ClientFunc builds a Strava client acting for the user with the given
// internal ID. A function, not an interface, for the same reason as
// worker.ClientFunc: (*strava.API).ForUser returns *strava.Client.
type ClientFunc func(userID int64, tok *oauth2.Token) backfill.Client

// Scheduler reconciles every member's recent activities: once when Run
// starts, then every interval.
type Scheduler struct {
	users      Users
	activities Activities
	newClient  ClientFunc
	logger     *slog.Logger
	now        func() time.Time
	// interval is the time between two passes.
	interval time.Duration
	// windowDays is how many days before today a pass goes back to.
	windowDays int
}

// NewScheduler returns a Scheduler that reconciles the last week, from 7 days
// before today (UTC) to today, every 24 hours.
func NewScheduler(users Users, activities Activities, newClient ClientFunc, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		users:      users,
		activities: activities,
		newClient:  newClient,
		logger:     logger,
		now:        time.Now,
		interval:   24 * time.Hour,
		windowDays: 7,
	}
}

// Run makes a pass at once, so that a restart recovers what was missed while
// the server was down, then one on every tick, until ctx is cancelled. It
// always returns nil: errors are logged, and the next pass retries.
// Cancellation stops a pass at its next Strava request.
func (s *Scheduler) Run(ctx context.Context) error {
	s.pass(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.pass(ctx)
		}
	}
}

// pass reconciles every member in turn, from windowDays before today (UTC)
// to today. A failure for one member is logged and the others go on. It
// stops before the next member when ctx is cancelled.
func (s *Scheduler) pass(ctx context.Context) {
	users, err := s.users.List(ctx)
	if err != nil {
		s.logger.Error("reconcile list users", "error", err)
		return
	}
	n := s.now().UTC()
	to := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, 0, -s.windowDays)
	first, last := from.Format(time.DateOnly), to.Format(time.DateOnly)
	s.logger.Info("reconcile pass", "users", len(users), "from", first, "to", last)

	for _, u := range users {
		if ctx.Err() != nil {
			return
		}
		client := s.newClient(u.ID, &oauth2.Token{
			AccessToken:  u.AccessToken,
			RefreshToken: u.RefreshToken,
			Expiry:       u.TokenExpiresAt,
		})
		res, err := User(ctx, client, s.activities, s.logger, u.ID, from, to)
		if err != nil {
			s.logger.Error("reconcile failed", "user_id", u.ID, "from", first, "to", last, "error", err)
			continue
		}
		s.logger.Info("reconcile done", "user_id", u.ID, "from", first, "to", last, "synced", res.Saved, "deleted", res.Deleted)
	}
}
