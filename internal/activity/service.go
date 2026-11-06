// Package activity stores Strava activities and keeps their points up to
// date. Every change goes through Service, which writes the activity and
// recomputes the points of each affected day (see scoring.Score) in one
// transaction, so stored points always match the stored activities.
//
// Callers are the webhook worker and the reconcile job. Both may repeat an
// operation: Save and Delete are idempotent.
package activity

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mario595/leaderboard/internal/scoring"
	"github.com/mario595/leaderboard/internal/store"
)

// Service saves and deletes activities, recomputing points per user and day.
// It is safe for concurrent use: each call runs in its own transaction, and
// the database allows one connection at a time.
type Service struct {
	db         *sql.DB
	activities *store.ActivityStore
}

// NewService returns a Service that stores activities in db.
func NewService(db *sql.DB) *Service {
	return &Service{
		db:         db,
		activities: store.NewActivityStore(db),
	}
}

// IDsBetween returns the Strava IDs of the user's activities whose local day is
// between from and to, both inclusive. See store.ActivityStore.IDsBetween.
func (s *Service) IDsBetween(ctx context.Context, userID int64, from, to time.Time) ([]int64, error) {
	return s.activities.IDsBetween(ctx, userID, from, to)
}

// Save stores a new activity or updates an existing one,
// then recomputes the points of every day the change affects
// (the old and the new day when the date changed), all in one transaction.
func (s *Service) Save(ctx context.Context, a store.Activity) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("save activity: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.activities.WithTx(tx)

	prev, err := q.Get(ctx, a.StravaID)
	found := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("save activity %d: %w", a.StravaID, err)
	}

	activity, err := q.Upsert(ctx, a)
	if err != nil {
		return fmt.Errorf("save activity %d: %w", a.StravaID, err)
	}

	err = recomputeDay(ctx, q, activity.UserID, activity.Day())
	if err != nil {
		return fmt.Errorf("save activity %d for user %d: %w", activity.StravaID, activity.UserID, err)
	}

	if found && prev.Day() != activity.Day() {
		err = recomputeDay(ctx, q, prev.UserID, prev.Day())
		if err != nil {
			return fmt.Errorf("save activity %d: %w", a.StravaID, err)
		}
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("save activity %d: %w", a.StravaID, err)
	}

	return nil
}

// Delete deletes an activity and recomputes points if needed in the same transaction.
// Unknown id does not return an error.
func (s *Service) Delete(ctx context.Context, stravaID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete activity %d: %w", stravaID, err)
	}
	defer func() { _ = tx.Rollback() }()

	q := s.activities.WithTx(tx)

	a, err := q.Get(ctx, stravaID)
	found := err == nil
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("delete activity %d: %w", stravaID, err)
	}

	if !found {
		return nil
	}

	if err := q.Delete(ctx, stravaID); err != nil {
		return fmt.Errorf("delete activity %d: %w", stravaID, err)
	}

	if err := recomputeDay(ctx, q, a.UserID, a.Day()); err != nil {
		return fmt.Errorf("delete activity %d: %w", stravaID, err)
	}

	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("delete activity %d: %w", stravaID, err)
	}

	return nil
}

func recomputeDay(ctx context.Context, q *store.ActivityStore, userID int64, day string) error {
	dayActivities, err := q.ListDay(ctx, userID, day)
	if err != nil {
		return fmt.Errorf("recompute day %q for user %d: %w", day, userID, err)
	}
	var activities []scoring.Activity
	for _, a := range dayActivities {
		activities = append(activities, toScoringActivity(a))
	}
	for _, award := range scoring.Score(activities) {
		err := q.SetPoints(ctx, award.ActivityID, award.Points)
		if err != nil {
			return fmt.Errorf("recompute day %q for user %d: %w", day, userID, err)
		}
	}

	return nil
}

func toScoringActivity(a store.Activity) scoring.Activity {
	return scoring.Activity{
		ID:          a.StravaID,
		SportType:   a.SportType,
		StartLocal:  a.StartLocal,
		MovingTime:  a.MovingTime,
		WorkoutType: a.WorkoutType,
		Description: a.Description,
	}
}
