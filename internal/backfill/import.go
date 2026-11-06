package backfill

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/mario595/leaderboard/internal/scoring"
	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
)

// Client lists and fetches one athlete's activities; *strava.Client
// satisfies it.
type Client interface {
	ListActivities(ctx context.Context, after, before time.Time, page, perPage int) ([]strava.Activity, error)
	Activity(ctx context.Context, id int64) (strava.Activity, error)
}

// ActivitySaver stores an activity and recomputes its day;
// *activity.Service satisfies it.
type ActivitySaver interface {
	Save(ctx context.Context, a store.Activity) error
}

// perPage is the largest page Strava's activity list allows.
const perPage = 200

// Result is what an import did.
type Result struct {
	// Details is the number of activities whose detail was fetched.
	Details int
	// IDs are the Strava IDs of the saved activities, in the order they were
	// saved.
	IDs []int64
}

// Import saves, for the user with the given internal ID, every activity whose
// local day is between from and to (both inclusive, only the dates count).
// It logs each page and detail fetch at Debug level. On error it stops and
// returns the counts so far; errors keep their type (*strava.RateLimitError,
// strava.ErrUnauthorized), so the caller can decide when to retry. An
// activity deleted between the list and its detail fetch is skipped.
//
// A webhook delete can race with an import that listed the activity a moment
// before, and the import then saves it again. This is rare and accepted: the
// reconcile job (#31) removes it.
func Import(ctx context.Context, c Client, saver ActivitySaver, logger *slog.Logger, userID int64, from, to time.Time) (Result, error) {
	after := from.AddDate(0, 0, -1)
	before := to.AddDate(0, 0, 2)
	first, last := from.Format(time.DateOnly), to.Format(time.DateOnly)
	var res Result
	for page := 1; ; page++ {
		list, err := c.ListActivities(ctx, after, before, page, perPage)
		if err != nil {
			return res, fmt.Errorf("import for user %d, list page %d: %w", userID, page, err)
		}
		inRange := 0
		for _, a := range list {
			day := a.StartLocal.Format(time.DateOnly)
			if day < first || day > last {
				continue
			}
			inRange++
			if isCandidate(a) {
				logger.Debug("backfill fetch detail", "user_id", userID, "activity_id", a.ID)
				res.Details++
				detail, err := c.Activity(ctx, a.ID)
				if errors.Is(err, strava.ErrNotFound) {
					logger.Info("backfill activity gone, skipped", "user_id", userID, "activity_id", a.ID)
					continue
				}
				if err != nil {
					return res, fmt.Errorf("import for user %d, fetch activity %d: %w", userID, a.ID, err)
				}
				a.Description = detail.Description
			}
			err = saver.Save(ctx, toStoreActivity(userID, a))
			if err != nil {
				return res, fmt.Errorf("import for user %d, save activity %d: %w", userID, a.ID, err)
			}
			res.IDs = append(res.IDs, a.ID)
		}
		logger.Debug("backfill page", "user_id", userID, "page", page, "listed", len(list), "in_range", inRange)
		if len(list) < perPage {
			break
		}
	}
	return res, nil
}

// toStoreActivity maps a Strava activity to the stored form for the user with
// the given internal ID.
func toStoreActivity(userID int64, a strava.Activity) store.Activity {
	return store.Activity{
		StravaID:    a.ID,
		UserID:      userID,
		SportType:   a.SportType,
		StartLocal:  a.StartLocal,
		MovingTime:  a.MovingTime,
		Distance:    a.Distance,
		WorkoutType: a.WorkoutType,
		Description: a.Description,
	}
}

// isCandidate reports whether the detail of a must be fetched: only the
// description is missing from a summary, and the #competition tag in it
// matters only for an activity that scores and is not already a race (a race
// scores 4 points; the tag cannot raise it).
func isCandidate(a strava.Activity) bool {
	s := scoring.Activity{
		ID:          a.ID,
		SportType:   a.SportType,
		StartLocal:  a.StartLocal,
		MovingTime:  a.MovingTime,
		WorkoutType: a.WorkoutType,
		Description: a.Description,
	}
	return s.Eligible() && !s.IsCompetition()
}
