package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
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

// ActivityCursor is a position in ListForUser's order (newest first): the next
// page holds the activities that come after it in that order, that is, those
// that started earlier, or at the same time with a lower Strava ID.
type ActivityCursor struct {
	StartLocal time.Time
	StravaID   int64
}

// defaultActivityCursor comes before every real activity in ListForUser's
// order, so the first page needs no separate query.
var defaultActivityCursor = ActivityCursor{
	StartLocal: time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC),
	StravaID:   math.MaxInt64,
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

const listDayQuery = `
	SELECT strava_id, user_id, sport_type, start_local, moving_time, distance, workout_type, description, points, created_at, updated_at
	FROM activities
	WHERE user_id = ? AND day = ?
	ORDER BY start_local, strava_id;
`

const setPointsQuery = `
	UPDATE activities
	SET points = ?
	WHERE strava_id = ?;
`

const deleteActivityQuery = `
	DELETE FROM activities
	WHERE strava_id = ?;
`

// listForUserQuery returns one page of a user's activities in a day range,
// newest first. The row-value comparison and the ORDER BY use the same key, in
// the same direction: the page starts right after the cursor.
const listForUserQuery = `
	SELECT strava_id, user_id, sport_type, start_local, moving_time, distance, workout_type, description, points, created_at, updated_at
	FROM activities
	WHERE user_id = ?
		AND  day BETWEEN ? AND ?
		AND (start_local, strava_id) < (?, ?)
	ORDER BY start_local DESC, strava_id DESC
	LIMIT ?;
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

// ListDay returns the user's activities on that day,
// ordered by start time then Strava ID. Empty slice if there are none.
func (s *ActivityStore) ListDay(ctx context.Context, userID int64, day string) ([]Activity, error) {
	result := []Activity{}
	rows, err := s.db.QueryContext(ctx, listDayQuery, userID, day)
	if err != nil {
		return nil, fmt.Errorf("list day %q for user %d: %w", day, userID, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		a, err := scanActivityRow(rows)
		if err != nil {
			return nil, fmt.Errorf("list day %q for user %d: %w", day, userID, err)
		}
		result = append(result, a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list day %q for user %d: %w", day, userID, err)
	}

	return result, nil
}

// SetPoints sets the points in a Strava Activity with stravaID without updating UpdatedAt.
// If the activity doesn't exist, this function does not return an error.
func (s *ActivityStore) SetPoints(ctx context.Context, stravaID int64, points int) error {
	_, err := s.db.ExecContext(ctx, setPointsQuery, points, stravaID)
	if err != nil {
		return fmt.Errorf("set points for activity %d: %w", stravaID, err)
	}

	return nil
}

// Delete deletes an activity. It is idempotent: a delete for an
// activity that doesn't exist doesn't return error.
func (s *ActivityStore) Delete(ctx context.Context, stravaID int64) error {
	_, err := s.db.ExecContext(ctx, deleteActivityQuery, stravaID)
	if err != nil {
		return fmt.Errorf("delete activity %d: %w", stravaID, err)
	}

	return nil
}

// ListForUser returns up to limit activities of the user whose local day lies
// between from and to (both inclusive), newest first, ties broken by Strava ID
// descending. after is the last activity of the previous page, or nil for the
// first page. Keyset pagination: rows arriving between two calls never shift
// the pages. Returns an empty, non-nil slice when nothing matches.
func (s *ActivityStore) ListForUser(ctx context.Context, userID int64, from, to time.Time, after *ActivityCursor, limit int) ([]Activity, error) {
	cursor := defaultActivityCursor
	if after != nil {
		cursor = *after
	}

	rows, err := s.db.QueryContext(ctx,
		listForUserQuery,
		userID,
		FormatDate(from),
		FormatDate(to),
		cursor.StartLocal.Format(startLocalLayout),
		cursor.StravaID,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list activities of user %d: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()

	result := []Activity{}
	for rows.Next() {
		r, err := scanActivityRow(rows)
		if err != nil {
			return nil, fmt.Errorf("list activities of user %d: %w", userID, err)
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list activities of user %d: %w", userID, err)
	}

	return result, nil

}

// idsBetweenQuery selects the Strava IDs of a user's activities whose day is in
// an inclusive range.
const idsBetweenQuery = `
	SELECT strava_id
	FROM activities
	WHERE user_id = ? AND day BETWEEN ? AND ?
	ORDER BY strava_id;
`

// IDsBetween returns the Strava IDs of the user's activities whose local day is
// from from to to, both inclusive, in ascending order. Only the dates of from
// and to count. The reconcile compares them with what Strava returns.
func (s *ActivityStore) IDsBetween(ctx context.Context, userID int64, from, to time.Time) ([]int64, error) {
	var result []int64
	fromDate := from.Format(time.DateOnly)
	toDate := to.Format(time.DateOnly)
	rows, err := s.db.QueryContext(ctx, idsBetweenQuery, userID, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("activity IDs of user %d from %s to %s: %w", userID, fromDate, toDate, err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var stravaID int64
		err := rows.Scan(&stravaID)
		if err != nil {
			return nil, fmt.Errorf("scan activity IDs of user %d from %s to %s: %w", userID, fromDate, toDate, err)
		}
		result = append(result, stravaID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan activity IDs of user %d from %s to %s: %w", userID, fromDate, toDate, err)
	}
	return result, nil
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
