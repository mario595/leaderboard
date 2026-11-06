package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mario595/leaderboard/internal/backfill"
	"github.com/mario595/leaderboard/internal/store"
)

// Activities saves, deletes and lists a member's stored activities;
// *activity.Service satisfies it. Save and Delete recompute the day's points.
type Activities interface {
	Save(ctx context.Context, a store.Activity) error
	Delete(ctx context.Context, stravaID int64) error
	IDsBetween(ctx context.Context, userID int64, from, to time.Time) ([]int64, error)
}

// Result is what a reconcile did.
type Result struct {
	// Saved is the number of activities saved, changed or not.
	Saved int
	// Deleted is the number of stored activities Strava no longer has.
	Deleted int
}

// User reconciles the activities of the user with the given internal ID whose
// local day is between from and to (both inclusive, only the dates count): it
// saves what Strava lists and deletes what is stored but no longer listed,
// including an activity deleted between the list and its detail fetch.
//
// On error it stops and returns the counts so far. An import error keeps its
// type (*strava.RateLimitError, strava.ErrUnauthorized) and nothing is
// deleted. A run is idempotent: the next one finishes the work.
func User(ctx context.Context, c backfill.Client, activities Activities, logger *slog.Logger, userID int64, from, to time.Time) (Result, error) {
	imported, err := backfill.Import(ctx, c, activities, logger, userID, from, to)
	res := Result{Saved: len(imported.IDs)}
	if err != nil {
		return res, fmt.Errorf("reconcile for user %d and dates from %s to %s: %w", userID, from.Format(time.DateOnly), to.Format(time.DateOnly), err)
	}

	ids, err := activities.IDsBetween(ctx, userID, from, to)
	if err != nil {
		return res, fmt.Errorf("reconcile for user %d and dates from %s to %s: %w", userID, from.Format(time.DateOnly), to.Format(time.DateOnly), err)
	}

	importIDs := make(map[int64]struct{}, len(imported.IDs))
	for _, id := range imported.IDs {
		importIDs[id] = struct{}{}
	}

	for _, id := range ids {
		if _, ok := importIDs[id]; ok {
			continue
		}
		if err := activities.Delete(ctx, id); err != nil {
			return res, fmt.Errorf("deleting activity %d for user %d: %w", id, userID, err)
		}
		logger.Info("reconcile deleted activity", "activity_id", id, "user_id", userID)
		res.Deleted++
	}
	return res, nil
}
