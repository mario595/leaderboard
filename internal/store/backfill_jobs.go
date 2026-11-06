package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// BackfillJob is a request to import one member's activities for one season.
// The backfill_jobs table is the queue, as webhook_events is for the worker.
type BackfillJob struct {
	// ID is the internal ID, assigned on insert.
	ID int64
	// UserID is the member whose activities are imported.
	UserID int64
	// SeasonID is the season whose date range is imported.
	SeasonID int64
	// Status is "pending" until the job is done or failed.
	Status string
	// Attempts is the number of previous failed attempts.
	Attempts int
	// LastError is the error of the last failed attempt, "" if none.
	LastError string
	// NotBefore is when the job may run (again).
	NotBefore time.Time
}

// BackfillStore persists backfill jobs in SQLite.
type BackfillStore struct {
	db  DBTX
	now func() time.Time
}

// NewBackfillStore returns a BackfillStore that runs its queries on db.
func NewBackfillStore(db DBTX) *BackfillStore {
	return &BackfillStore{
		db:  db,
		now: time.Now,
	}
}

// enqueueQuery inserts a pending job, or sets a finished (done or failed) one
// back to pending as new. A job that is already pending is left alone: the
// WHERE of DO UPDATE filters the existing row, and no row changes.
const enqueueQuery = `
	INSERT INTO backfill_jobs (user_id, season_id, created_at)
	VALUES (?, ?, ?)
	ON CONFLICT (user_id, season_id) DO UPDATE SET
		status = 'pending',
		attempts = 0,
		last_error = '',
		not_before = 0,
		done_at = NULL
	WHERE status != 'pending';
`

// enqueueAllQuery is enqueueQuery for every user. SQLite needs the WHERE true:
// without it, it parses ON CONFLICT as the join condition of the SELECT.
const enqueueAllQuery = `
	INSERT INTO backfill_jobs (user_id, season_id, created_at)
	SELECT id, ?, ? FROM users WHERE true
	ON CONFLICT (user_id, season_id) DO UPDATE SET
		status = 'pending',
		attempts = 0,
		last_error = '',
		not_before = 0,
		done_at = NULL
	WHERE status != 'pending';
`

// nextDueBackfillQuery finds the oldest pending job whose time has come.
const nextDueBackfillQuery = `
	SELECT id, user_id, season_id, status, attempts, last_error, not_before
	FROM backfill_jobs
	WHERE status = 'pending' AND not_before <= ?
	ORDER BY id
	LIMIT 1;
`

// markBackfillDoneQuery marks a job done.
const markBackfillDoneQuery = `
	UPDATE backfill_jobs
	SET status = 'done',
		done_at = ?
	WHERE id = ?;
`

// markBackfillRetryQuery counts a failed attempt and keeps the job pending
// until not_before.
const markBackfillRetryQuery = `
	UPDATE backfill_jobs
	SET attempts = attempts + 1,
		last_error = ?,
		not_before = ?
	WHERE id = ?;
`

// markBackfillFailedQuery counts a final failed attempt and marks the job failed.
const markBackfillFailedQuery = `
	UPDATE backfill_jobs
	SET attempts = attempts + 1,
		status = 'failed',
		last_error = ?,
		done_at = ?
	WHERE id = ?;
`

// Enqueue queues a backfill of the user's activities for the season. It
// returns false, with a nil error, when that job is already pending. A done or
// failed job is set back to pending, as new: that is how a season is imported
// again.
func (s *BackfillStore) Enqueue(ctx context.Context, userID, seasonID int64) (bool, error) {
	r, err := s.db.ExecContext(ctx, enqueueQuery, userID, seasonID, s.now().Unix())
	if err != nil {
		return false, fmt.Errorf("enqueue backfill for user %d, season %d: %w", userID, seasonID, err)
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("enqueue backfill for user %d, season %d: %w", userID, seasonID, err)
	}

	return affected > 0, nil
}

// EnqueueAll does Enqueue for every user and the season, in one statement. It
// returns how many jobs were queued or reset; users whose job is already
// pending are not counted.
func (s *BackfillStore) EnqueueAll(ctx context.Context, seasonID int64) (int, error) {
	r, err := s.db.ExecContext(ctx, enqueueAllQuery, seasonID, s.now().Unix())
	if err != nil {
		return 0, fmt.Errorf("enqueue backfill for all users, season %d: %w", seasonID, err)
	}
	affected, err := r.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("enqueue backfill for all users, season %d: %w", seasonID, err)
	}

	return int(affected), nil
}

// NextDue returns the oldest pending job that may run now. found is false,
// with a nil error, when nothing is due.
func (s *BackfillStore) NextDue(ctx context.Context) (BackfillJob, bool, error) {
	r := s.db.QueryRowContext(ctx, nextDueBackfillQuery, s.now().Unix())
	job, err := scanBackfillJobRow(r)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return BackfillJob{}, false, nil
		}
		return BackfillJob{}, false, fmt.Errorf("next due backfill: %w", err)
	}
	return job, true, nil
}

// MarkDone records that the job finished.
func (s *BackfillStore) MarkDone(ctx context.Context, id int64) error {
	if _, err := s.db.ExecContext(ctx, markBackfillDoneQuery, s.now().Unix(), id); err != nil {
		return fmt.Errorf("mark backfill %d done: %w", id, err)
	}
	return nil
}

// MarkRetry records a failed attempt: it counts it, keeps errMsg, and leaves
// the job pending until notBefore.
func (s *BackfillStore) MarkRetry(ctx context.Context, id int64, errMsg string, notBefore time.Time) error {
	if _, err := s.db.ExecContext(ctx, markBackfillRetryQuery, errMsg, notBefore.Unix(), id); err != nil {
		return fmt.Errorf("mark backfill %d for retry: %w", id, err)
	}
	return nil
}

// MarkFailed records a final failure: the job will not run again unless it is
// enqueued again. It counts the attempt and keeps errMsg.
func (s *BackfillStore) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	if _, err := s.db.ExecContext(ctx, markBackfillFailedQuery, errMsg, s.now().Unix(), id); err != nil {
		return fmt.Errorf("mark backfill %d failed: %w", id, err)
	}
	return nil
}

// scanBackfillJobRow scans one row in the column order of nextDueBackfillQuery.
// sql.ErrNoRows is returned wrapped: whether no row is an error is the
// caller's decision.
func scanBackfillJobRow(r rowScanner) (BackfillJob, error) {
	var job BackfillJob
	var notBefore int64
	err := r.Scan(
		&job.ID,
		&job.UserID,
		&job.SeasonID,
		&job.Status,
		&job.Attempts,
		&job.LastError,
		&notBefore,
	)
	if err != nil {
		return BackfillJob{}, fmt.Errorf("scan backfill job: %w", err)
	}
	job.NotBefore = time.Unix(notBefore, 0)
	return job, nil
}
