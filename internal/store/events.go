package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Event is a Strava webhook event, stored as received and processed later by
// the worker. The webhook_events table is the queue: an event the handler
// acknowledged to Strava always has a row here.
type Event struct {
	// ID is the internal ID, assigned on insert.
	ID int64
	// ObjectType is "activity" or "athlete".
	ObjectType string
	// ObjectID is the Strava activity ID or athlete ID.
	ObjectID int64
	// AspectType is "create", "update" or "delete".
	AspectType string
	// OwnerID is the Strava athlete ID of the owner.
	OwnerID int64
	// EventTime is when the change happened on Strava.
	EventTime time.Time
	// Updates is the raw JSON object Strava sent with an update, for example
	// {"title":"Morning Run"} or {"authorized":"false"}; "{}" otherwise.
	Updates string
	// Status is "pending" until the worker has processed the event.
	Status string
	// ReceivedAt is when the handler stored the event.
	ReceivedAt time.Time
	// NextAttemptAt is when the event should next be handled again.
	NextAttemptAt time.Time
	// Attempts is the number of previous attempts for this event.
	Attempts int64
}

// EventStore persists webhook events in SQLite.
type EventStore struct {
	db  DBTX
	now func() time.Time
}

// NewEventStore returns an EventStore that runs its queries on db.
func NewEventStore(db DBTX) *EventStore {
	return &EventStore{
		db:  db,
		now: time.Now,
	}
}

// insertEventQuery is the sql query to insert a new event.
// It doesn't do anything on conflict.
const insertEventQuery = `
	INSERT INTO webhook_events (object_type, object_id, aspect_type,
		owner_id, event_time, updates, received_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(object_type, object_id, aspect_type, event_time) DO NOTHING;
`

// getNextDueQuery finds the next event that needs
// processing (status is pending and the next_attempt_at is in the past).
// It returns only the one with the lowest ID.
const getNextDueQuery = `
	SELECT id, object_type, object_id, aspect_type, owner_id,
		event_time, updates, status, attempts, received_at, next_attempt_at
	FROM webhook_events
	WHERE status = 'pending' AND next_attempt_at <= ?
	ORDER BY id
	LIMIT 1;
`

// setStatusQuery updates the event to the status passed.
const setStatusQuery = `
	UPDATE webhook_events
	SET status = ?,
		processed_at = ?
	WHERE id = ?;
`

// markAsRetryQuery sets last_error, attempts and next_attempt_at leaving status as pending.
const markAsRetryQuery = `
	UPDATE webhook_events
	SET attempts = attempts + 1,
		last_error = ?,
		next_attempt_at = ?
	WHERE id = ?
	;
`

// markAsFailedQuery sets the status to failed, increments attempts, sets last_error and processed_at.
const markAsFailedQuery = `
	UPDATE webhook_events
	SET attempts = attempts + 1,
		status = 'failed',
		last_error = ?,
		processed_at = ?
	WHERE id = ?;
`

// Insert stores e as a pending event. A duplicate of an event already stored
// (same object, aspect type and event time) is not an error: Strava retries
// deliveries, so Insert returns inserted == false and stores nothing.
func (s *EventStore) Insert(ctx context.Context, e Event) (inserted bool, err error) {
	r, err := s.db.ExecContext(
		ctx,
		insertEventQuery,
		e.ObjectType,
		e.ObjectID,
		e.AspectType,
		e.OwnerID,
		e.EventTime.Unix(),
		e.Updates,
		s.now().Unix(),
	)
	if err != nil {
		return false, fmt.Errorf("insert %s %s event for object %d: %w", e.AspectType, e.ObjectType, e.ObjectID, err)
	}

	affected, err := r.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("insert %s %s event for object %d: %w", e.AspectType, e.ObjectType, e.ObjectID, err)
	}
	return affected == 1, nil
}

// NextDue returns the oldest pending event whose next attempt is due now.
// found is false, with a nil error, when nothing is due: the queue is empty or
// every pending event is waiting for a retry.
func (s *EventStore) NextDue(ctx context.Context) (Event, bool, error) {
	r := s.db.QueryRowContext(ctx, getNextDueQuery, s.now().Unix())
	e, err := scanEventRow(r)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, false, nil
		}
		return Event{}, false, fmt.Errorf("next due: %w", err)
	}
	return e, true, nil
}

// MarkDone records that the event was processed. An unknown id is not an
// error: the worker only marks events it got from NextDue.
func (s *EventStore) MarkDone(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, setStatusQuery, "done", s.now().Unix(), id)
	if err != nil {
		return fmt.Errorf("mark event %d as done: %w", id, err)
	}
	return nil
}

// MarkRetry records a failed attempt: it counts the attempt, keeps errMsg,
// and leaves the event pending until next.
func (s *EventStore) MarkRetry(ctx context.Context, id int64, errMsg string, next time.Time) error {
	_, err := s.db.ExecContext(ctx, markAsRetryQuery, errMsg, next.Unix(), id)
	if err != nil {
		return fmt.Errorf("mark as retry event %d: %w", id, err)
	}
	return nil
}

// MarkFailed records a final failure: the event will not be retried. It
// counts the attempt and keeps errMsg for inspection.
func (s *EventStore) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	_, err := s.db.ExecContext(ctx, markAsFailedQuery, errMsg, s.now().Unix(), id)
	if err != nil {
		return fmt.Errorf("mark as failed event %d: %w", id, err)
	}
	return nil
}

// scanEventRow scans one row in the column order of getNextDueQuery and
// converts unix seconds back to time.Time. sql.ErrNoRows is returned wrapped:
// whether no row is an error is the caller's decision.
func scanEventRow(r rowScanner) (Event, error) {
	var event Event
	var eventTime, receivedAt, nextAttemptAt int64
	err := r.Scan(
		&event.ID,
		&event.ObjectType,
		&event.ObjectID,
		&event.AspectType,
		&event.OwnerID,
		&eventTime,
		&event.Updates,
		&event.Status,
		&event.Attempts,
		&receivedAt,
		&nextAttemptAt,
	)
	if err != nil {
		return Event{}, fmt.Errorf("scan event row: %w", err)
	}
	event.EventTime = time.Unix(eventTime, 0)
	event.ReceivedAt = time.Unix(receivedAt, 0)
	event.NextAttemptAt = time.Unix(nextAttemptAt, 0)
	return event, nil
}
