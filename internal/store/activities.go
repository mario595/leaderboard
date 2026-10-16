package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// startLocalLayout is how start_local is stored: local wall-clock time, no zone.
const startLocalLayout = "2006-01-02T15:04:05"

// Activity is a stored Strava activity with the points awarded to it.
type Activity struct {
	// StravaID is the Strava activity ID and the primary key.
	StravaID int64
	// UserID is the internal ID of the owning user.
	UserID int64
	// SportType is the Strava sport_type, for example "Run".
	SportType string
	// StartLocal is the athlete's local wall-clock start time (Strava
	// start_date_local). Stored without a zone; read back as UTC.
	StartLocal time.Time
	// MovingTime is the Strava moving_time, stored in whole seconds.
	MovingTime time.Duration
	// Distance is the Strava distance in metres.
	Distance float64
	// WorkoutType is the Strava workout_type, 0 when Strava returns null.
	WorkoutType int
	// Description is the Strava activity description. Used only for scoring;
	// never shown to other users.
	Description string
	// Points are the points awarded after the daily cap. Set by the
	// recompute (SetPoints), never by Upsert.
	Points int
	// CreatedAt is when the activity was first stored.
	CreatedAt time.Time
	// UpdatedAt is when the activity was last changed from Strava data.
	UpdatedAt time.Time
}

// Day returns the local start date as YYYY-MM-DD, the value stored in the
// day column.
func (a Activity) Day() string {
	return a.StartLocal.Format(time.DateOnly)
}

// ActivityStore persists activities in SQLite.
type ActivityStore struct {
	db  DBTX
	now func() time.Time
}

// NewActivityStore returns an ActivityStore that runs its queries on db.
func NewActivityStore(db DBTX) *ActivityStore {
	return &ActivityStore{
		db:  db,
		now: time.Now,
	}
}

// WithTx returns a copy of s that runs its queries in tx. s itself is not
// changed.
func (s *ActivityStore) WithTx(tx *sql.Tx) *ActivityStore {
	return &ActivityStore{
		db:  tx,
		now: s.now,
	}
}

// upsertActivityQuery inserts an activity with 0 points, or updates the
// Strava fields of an existing one. points, created_at and user_id are not in
// the update clause, so an update keeps them.
const upsertActivityQuery = `
	INSERT INTO activities (strava_id, user_id, sport_type, start_local, day, moving_time, distance, workout_type, description, points, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
	ON CONFLICT(strava_id) DO UPDATE SET
		sport_type = excluded.sport_type,
		start_local = excluded.start_local,
		moving_time = excluded.moving_time,
		distance = excluded.distance,
		day = excluded.day,
		workout_type = excluded.workout_type,
		description = excluded.description,
		updated_at = excluded.updated_at
	RETURNING strava_id, user_id, sport_type, start_local, moving_time, distance, workout_type, description, points, created_at, updated_at;
`

const getActivityQuery = `
	SELECT strava_id, user_id, sport_type, start_local, moving_time, distance, workout_type, description, points, created_at, updated_at
	FROM activities
	WHERE strava_id = ?;
`

// Upsert inserts a, or updates the stored activity with the same StravaID,
// and returns the stored row. a.Points and a.CreatedAt are ignored: a new
// activity starts with 0 points, and an update keeps the stored points,
// creation time and owner. The day column is computed from a.StartLocal.
func (s *ActivityStore) Upsert(ctx context.Context, a Activity) (Activity, error) {
	now := s.now()
	r := s.db.QueryRowContext(
		ctx,
		upsertActivityQuery,
		a.StravaID,
		a.UserID,
		a.SportType,
		a.StartLocal.Format(startLocalLayout),
		a.Day(),
		int64(a.MovingTime/time.Second),
		a.Distance,
		a.WorkoutType,
		a.Description,
		now.Unix(),
		now.Unix(),
	)

	activity, err := scanActivityRow(r)
	if err != nil {
		return Activity{}, fmt.Errorf("upsert activity %d: %w", a.StravaID, err)
	}

	return activity, nil
}

// Get returns the activity with the given Strava ID, or ErrNotFound.
func (s *ActivityStore) Get(ctx context.Context, stravaID int64) (Activity, error) {
	r := s.db.QueryRowContext(ctx, getActivityQuery, stravaID)
	activity, err := scanActivityRow(r)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Activity{}, ErrNotFound
		}
		return Activity{}, fmt.Errorf("get activity %d: %w", stravaID, err)
	}
	return activity, nil
}

// scanActivityRow scans one row in the column order of upsertActivityQuery's
// RETURNING clause and converts the stored formats back: unix seconds to
// time.Time, seconds to time.Duration, and start_local text to a UTC time.
func scanActivityRow(r rowScanner) (Activity, error) {
	var activity Activity
	var updatedAt, createdAt, movingTime int64
	var startLocal string

	err := r.Scan(
		&activity.StravaID,
		&activity.UserID,
		&activity.SportType,
		&startLocal,
		&movingTime,
		&activity.Distance,
		&activity.WorkoutType,
		&activity.Description,
		&activity.Points,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return Activity{}, fmt.Errorf("scan activity: %w", err)
	}

	activity.CreatedAt = time.Unix(createdAt, 0)
	activity.UpdatedAt = time.Unix(updatedAt, 0)
	activity.MovingTime = time.Duration(movingTime) * time.Second
	parsedStartLocal, err := time.Parse(startLocalLayout, startLocal)
	if err != nil {
		return Activity{}, fmt.Errorf("parse start_local %q: %w", startLocal, err)
	}
	activity.StartLocal = parsedStartLocal

	return activity, nil
}
