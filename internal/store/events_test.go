package store

import (
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestEventStore_insert(t *testing.T) {
	db := newTestDB(t)
	s := NewEventStore(db)
	received := time.Date(2026, 10, 15, 12, 0, 1, 0, time.UTC)
	s.now = func() time.Time { return received }

	e := Event{
		ObjectType: "activity",
		ObjectID:   12345678901,
		AspectType: "update",
		OwnerID:    134815,
		EventTime:  time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Updates:    `{"title":"Morning Run"}`,
	}
	inserted, err := s.Insert(t.Context(), e)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	testutil.AssertEqual(t, "inserted", true, inserted)

	// No reader exists yet (#14 adds one), so check the row with plain SQL.
	var (
		objectType, aspectType, updates, status, lastError string
		objectID, ownerID, eventTime, receivedAt           int64
		attempts                                           int
	)
	err = db.QueryRowContext(t.Context(), `
		SELECT object_type, object_id, aspect_type, owner_id, event_time, updates,
		       status, attempts, last_error, received_at
		FROM webhook_events`).Scan(
		&objectType, &objectID, &aspectType, &ownerID, &eventTime, &updates,
		&status, &attempts, &lastError, &receivedAt)
	if err != nil {
		t.Fatalf("select event: %v", err)
	}
	testutil.AssertEqual(t, "object_type", "activity", objectType)
	testutil.AssertEqual(t, "object_id", int64(12345678901), objectID)
	testutil.AssertEqual(t, "aspect_type", "update", aspectType)
	testutil.AssertEqual(t, "owner_id", int64(134815), ownerID)
	testutil.AssertEqual(t, "event_time", e.EventTime.Unix(), eventTime)
	testutil.AssertEqual(t, "updates", `{"title":"Morning Run"}`, updates)
	testutil.AssertEqual(t, "status", "pending", status)
	testutil.AssertEqual(t, "attempts", 0, attempts)
	testutil.AssertEqual(t, "last_error", "", lastError)
	testutil.AssertEqual(t, "received_at", received.Unix(), receivedAt)
}

func TestEventStore_insertDuplicate(t *testing.T) {
	db := newTestDB(t)
	s := NewEventStore(db)
	e := Event{
		ObjectType: "activity",
		ObjectID:   12345678901,
		AspectType: "create",
		OwnerID:    134815,
		EventTime:  time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Updates:    "{}",
	}

	if _, err := s.Insert(t.Context(), e); err != nil {
		t.Fatalf("first Insert: %v", err)
	}
	// Strava retried the same event: not an error, but nothing new stored.
	inserted, err := s.Insert(t.Context(), e)
	if err != nil {
		t.Fatalf("duplicate Insert: %v", err)
	}
	testutil.AssertEqual(t, "duplicate inserted", false, inserted)

	// Positive control for the unique key: the same activity with another
	// aspect type is a different event.
	e.AspectType = "update"
	inserted, err = s.Insert(t.Context(), e)
	if err != nil {
		t.Fatalf("Insert update event: %v", err)
	}
	testutil.AssertEqual(t, "update event inserted", true, inserted)

	var count int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM webhook_events").Scan(&count); err != nil {
		t.Fatalf("count events: %v", err)
	}
	testutil.AssertEqual(t, "rows", 2, count)
}
