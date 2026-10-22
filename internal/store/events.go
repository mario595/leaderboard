package store

import (
	"context"
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

const insertEventQuery = `
	INSERT INTO webhook_events (object_type, object_id, aspect_type, owner_id, event_time, updates, received_at)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(object_type, object_id, aspect_type, event_time) DO NOTHING;
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
