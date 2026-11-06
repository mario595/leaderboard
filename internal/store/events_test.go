package store

import (
	"database/sql"
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

func TestEventStore_nextDue(t *testing.T) {
	db := newTestDB(t)
	s := NewEventStore(db)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }

	if _, found, err := s.NextDue(t.Context()); err != nil || found {
		t.Fatalf("NextDue on an empty queue: found=%v err=%v, wanted false and nil", found, err)
	}

	first := insertTestEvent(t, s, 1, "create")
	second := insertTestEvent(t, s, 2, "update")
	insertTestEvent(t, s, 3, "update")

	// Oldest first.
	e, found, err := s.NextDue(t.Context())
	if err != nil || !found {
		t.Fatalf("NextDue: found=%v err=%v", found, err)
	}
	testutil.AssertEqual(t, "next ID", first, e.ID)
	testutil.AssertEqual(t, "ObjectType", "activity", e.ObjectType)
	testutil.AssertEqual(t, "ObjectID", int64(1), e.ObjectID)
	testutil.AssertEqual(t, "AspectType", "create", e.AspectType)
	testutil.AssertEqual(t, "OwnerID", int64(134815), e.OwnerID)
	testutil.AssertTime(t, "EventTime", time.Date(2026, 10, 15, 11, 0, 0, 0, time.UTC), e.EventTime)
	testutil.AssertEqual(t, "Updates", `{"title":"x"}`, e.Updates)
	testutil.AssertEqual(t, "Status", "pending", e.Status)
	testutil.AssertEqual(t, "Attempts", 0, e.Attempts)

	// Done and failed events are no longer due; a retry is due only from its time.
	if err := s.MarkDone(t.Context(), first); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	if err := s.MarkRetry(t.Context(), second, "strava GET: status 500", now.Add(5*time.Minute)); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}
	e, _, err = s.NextDue(t.Context())
	if err != nil {
		t.Fatalf("NextDue after marking: %v", err)
	}
	testutil.AssertEqual(t, "next ID skips done and not-yet-due", int64(3), e.ObjectID)

	// Five minutes later the retry is due again, and it is older than event 3.
	s.now = func() time.Time { return now.Add(5 * time.Minute) }
	e, _, err = s.NextDue(t.Context())
	if err != nil {
		t.Fatalf("NextDue after the retry time: %v", err)
	}
	testutil.AssertEqual(t, "retried event is due", second, e.ID)
	testutil.AssertEqual(t, "Attempts after one retry", 1, e.Attempts)
	testutil.AssertTime(t, "NextAttemptAt", now.Add(5*time.Minute), e.NextAttemptAt)
}

func TestEventStore_markDoneAndFailed(t *testing.T) {
	db := newTestDB(t)
	s := NewEventStore(db)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }

	done := insertTestEvent(t, s, 1, "create")
	failed := insertTestEvent(t, s, 2, "create")

	if err := s.MarkDone(t.Context(), done); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	if err := s.MarkFailed(t.Context(), failed, "strava: unauthorized"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	type row struct {
		status, lastError string
		attempts          int
		processedAt       sql.NullInt64
	}
	read := func(id int64) row {
		t.Helper()
		var r row
		err := db.QueryRowContext(t.Context(),
			"SELECT status, last_error, attempts, processed_at FROM webhook_events WHERE id = ?", id,
		).Scan(&r.status, &r.lastError, &r.attempts, &r.processedAt)
		if err != nil {
			t.Fatalf("select event %d: %v", id, err)
		}
		return r
	}

	d := read(done)
	testutil.AssertEqual(t, "done status", "done", d.status)
	testutil.AssertEqual(t, "done processed_at", now.Unix(), d.processedAt.Int64)
	testutil.AssertEqual(t, "done attempts", 0, d.attempts)

	f := read(failed)
	testutil.AssertEqual(t, "failed status", "failed", f.status)
	testutil.AssertEqual(t, "failed last_error", "strava: unauthorized", f.lastError)
	testutil.AssertEqual(t, "failed attempts", 1, f.attempts)
	testutil.AssertEqual(t, "failed processed_at", now.Unix(), f.processedAt.Int64)

	// Neither is due any more.
	if _, found, err := s.NextDue(t.Context()); err != nil || found {
		t.Errorf("NextDue with only done and failed events: found=%v err=%v", found, err)
	}
}

// A recheck is a copy of the create event, due later: the worker handles it
// like any activity event.
func TestEventStore_scheduleRecheck(t *testing.T) {
	db := newTestDB(t)
	s := NewEventStore(db)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	insertTestEvent(t, s, 1, "create")
	create, found, err := s.NextDue(t.Context())
	if err != nil || !found {
		t.Fatalf("NextDue: found=%v err=%v", found, err)
	}
	if err := s.MarkDone(t.Context(), create.ID); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	at := now.Add(2 * time.Hour)

	if err := s.ScheduleRecheck(t.Context(), create, at); err != nil {
		t.Fatalf("ScheduleRecheck: %v", err)
	}

	// Not due before its time.
	if e, found, err := s.NextDue(t.Context()); err != nil || found {
		t.Fatalf("NextDue before the recheck time: got event %d (found=%v, err=%v), want nothing", e.ID, found, err)
	}
	now = at
	recheck, found, err := s.NextDue(t.Context())
	if err != nil || !found {
		t.Fatalf("NextDue at the recheck time: found=%v err=%v", found, err)
	}
	if recheck.ID == create.ID {
		t.Error("the recheck reused the create's row, want a new event")
	}
	testutil.AssertEqual(t, "ObjectType", "activity", recheck.ObjectType)
	testutil.AssertEqual(t, "ObjectID", create.ObjectID, recheck.ObjectID)
	testutil.AssertEqual(t, "OwnerID", create.OwnerID, recheck.OwnerID)
	testutil.AssertEqual(t, "AspectType", "recheck", recheck.AspectType)
	testutil.AssertTime(t, "EventTime", create.EventTime, recheck.EventTime)
	// The create's updates are not copied: a recheck has none.
	testutil.AssertEqual(t, "Updates", "{}", recheck.Updates)
	testutil.AssertEqual(t, "Status", "pending", recheck.Status)
	testutil.AssertEqual(t, "Attempts", 0, recheck.Attempts)
}

// The same create processed twice (a Strava retry, or a worker retry after a
// failed mark) schedules one recheck: same object and event time.
func TestEventStore_scheduleRecheckTwice(t *testing.T) {
	db := newTestDB(t)
	s := NewEventStore(db)
	insertTestEvent(t, s, 1, "create")
	create, _, err := s.NextDue(t.Context())
	if err != nil {
		t.Fatalf("NextDue: %v", err)
	}

	for i := range 2 {
		at := s.now().Add(time.Duration(2+i) * time.Hour)
		if err := s.ScheduleRecheck(t.Context(), create, at); err != nil {
			t.Fatalf("ScheduleRecheck %d: %v", i+1, err)
		}
	}

	var n int
	if err := db.QueryRowContext(t.Context(),
		"SELECT COUNT(*) FROM webhook_events WHERE aspect_type = 'recheck'").Scan(&n); err != nil {
		t.Fatalf("count rechecks: %v", err)
	}
	testutil.AssertEqual(t, "rechecks", 1, n)
}

// insertTestEvent stores a pending activity event for objectID and returns
// its ID.
func insertTestEvent(t *testing.T, s *EventStore, objectID int64, aspect string) int64 {
	t.Helper()
	inserted, err := s.Insert(t.Context(), Event{
		ObjectType: "activity",
		ObjectID:   objectID,
		AspectType: aspect,
		OwnerID:    134815,
		EventTime:  time.Date(2026, 10, 15, 11, 0, 0, 0, time.UTC),
		Updates:    `{"title":"x"}`,
	})
	if err != nil || !inserted {
		t.Fatalf("insert event %d: inserted=%v err=%v", objectID, inserted, err)
	}
	var id int64
	if err := s.db.QueryRowContext(t.Context(),
		"SELECT id FROM webhook_events WHERE object_id = ? AND aspect_type = ?", objectID, aspect,
	).Scan(&id); err != nil {
		t.Fatalf("read id of event %d: %v", objectID, err)
	}
	return id
}
