package store

import (
	"database/sql"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestActivityStore_upsertAndGet(t *testing.T) {
	s, db, userID := newTestActivityStore(t)
	created := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return created }

	a := Activity{
		StravaID:    1001,
		UserID:      userID,
		SportType:   "TrailRun",
		StartLocal:  time.Date(2026, 10, 15, 23, 30, 15, 0, time.UTC),
		MovingTime:  95*time.Minute + 7*time.Second,
		Distance:    15234.5,
		WorkoutType: 1,
		Description: "Night trail #competition",
		Points:      4, // ignored by Upsert
	}

	returned, err := s.Upsert(t.Context(), a)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := s.Get(t.Context(), a.StravaID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// Upsert's RETURNING and Get's SELECT must scan to the same value.
	if returned != got {
		t.Errorf("Upsert returned %+v, Get returned %+v", returned, got)
	}

	testutil.AssertEqual(t, "StravaID", a.StravaID, got.StravaID)
	testutil.AssertEqual(t, "UserID", a.UserID, got.UserID)
	testutil.AssertEqual(t, "SportType", a.SportType, got.SportType)
	testutil.AssertTime(t, "StartLocal", a.StartLocal, got.StartLocal)
	testutil.AssertEqual(t, "StartLocal location", time.UTC, got.StartLocal.Location())
	testutil.AssertEqual(t, "MovingTime", a.MovingTime, got.MovingTime)
	testutil.AssertEqual(t, "Distance", a.Distance, got.Distance)
	testutil.AssertEqual(t, "WorkoutType", a.WorkoutType, got.WorkoutType)
	testutil.AssertEqual(t, "Description", a.Description, got.Description)
	testutil.AssertEqual(t, "Points", 0, got.Points)
	testutil.AssertTime(t, "CreatedAt", created, got.CreatedAt)
	testutil.AssertTime(t, "UpdatedAt", created, got.UpdatedAt)

	// The stored formats are a contract with other queries (ListDay, the
	// leaderboard), so check the raw columns too, not only the round trip.
	var startLocal, day string
	var movingTime int64
	err = db.QueryRowContext(t.Context(),
		"SELECT start_local, day, moving_time FROM activities WHERE strava_id = ?", a.StravaID,
	).Scan(&startLocal, &day, &movingTime)
	if err != nil {
		t.Fatalf("select raw columns: %v", err)
	}
	testutil.AssertEqual(t, "start_local column", "2026-10-15T23:30:15", startLocal)
	testutil.AssertEqual(t, "day column", "2026-10-15", day)
	testutil.AssertEqual(t, "moving_time column", int64(95*60+7), movingTime)
}

func TestActivityStore_upsertExistingKeepsPointsAndCreatedAt(t *testing.T) {
	s, db, userID := newTestActivityStore(t)
	created := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	updated := created.Add(time.Hour)
	s.now = func() time.Time { return created }

	a := Activity{
		StravaID:   1001,
		UserID:     userID,
		SportType:  "Run",
		StartLocal: time.Date(2026, 10, 15, 7, 30, 0, 0, time.UTC),
		MovingTime: 45 * time.Minute,
		Distance:   9000,
	}
	if _, err := s.Upsert(t.Context(), a); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	// Points are written by the recompute, never by Upsert. Set them directly.
	if _, err := db.ExecContext(t.Context(), "UPDATE activities SET points = 4 WHERE strava_id = ?", a.StravaID); err != nil {
		t.Fatalf("set points: %v", err)
	}

	// The athlete edits the activity on Strava: every Strava field changes,
	// including the date.
	s.now = func() time.Time { return updated }
	edited := Activity{
		StravaID:    a.StravaID,
		UserID:      a.UserID,
		SportType:   "Ride",
		StartLocal:  time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC),
		MovingTime:  2 * time.Hour,
		Distance:    60000,
		WorkoutType: 11,
		Description: "Edited",
		Points:      0, // ignored by Upsert: must not reset the stored 4
	}
	returned, err := s.Upsert(t.Context(), edited)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	got, err := s.Get(t.Context(), a.StravaID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if returned != got {
		t.Errorf("Upsert returned %+v, Get returned %+v", returned, got)
	}

	testutil.AssertEqual(t, "SportType", edited.SportType, got.SportType)
	testutil.AssertTime(t, "StartLocal", edited.StartLocal, got.StartLocal)
	testutil.AssertEqual(t, "MovingTime", edited.MovingTime, got.MovingTime)
	testutil.AssertEqual(t, "Distance", edited.Distance, got.Distance)
	testutil.AssertEqual(t, "WorkoutType", edited.WorkoutType, got.WorkoutType)
	testutil.AssertEqual(t, "Description", edited.Description, got.Description)
	testutil.AssertEqual(t, "Points", 4, got.Points)
	testutil.AssertTime(t, "CreatedAt", created, got.CreatedAt)
	testutil.AssertTime(t, "UpdatedAt", updated, got.UpdatedAt)

	var day string
	if err := db.QueryRowContext(t.Context(), "SELECT day FROM activities WHERE strava_id = ?", a.StravaID).Scan(&day); err != nil {
		t.Fatalf("select day: %v", err)
	}
	testutil.AssertEqual(t, "day column", "2026-10-16", day)
}

func TestActivityStore_getUnknown(t *testing.T) {
	s, _, _ := newTestActivityStore(t)

	_, err := s.Get(t.Context(), 404)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get unknown activity: got error %v, wanted ErrNotFound", err)
	}
}

func TestActivityStore_upsertUnknownUser(t *testing.T) {
	s, _, userID := newTestActivityStore(t)

	a := Activity{
		StravaID:   1001,
		UserID:     userID + 1, // no such user
		SportType:  "Run",
		StartLocal: time.Date(2026, 10, 15, 7, 30, 0, 0, time.UTC),
		MovingTime: 45 * time.Minute,
	}
	// Fails only if the foreign_keys pragma is on: SQLite does not enforce
	// foreign keys by default.
	if _, err := s.Upsert(t.Context(), a); err == nil {
		t.Errorf("Upsert with unknown user: got nil error, wanted a foreign key error")
	}
}

func TestActivityStore_listDay(t *testing.T) {
	s, db, userID := newTestActivityStore(t)
	otherUserID := createTestUser(t, db, 43)

	// Inserted out of order: the result must be sorted by start time, then
	// Strava ID (activities 1 and 2 start at the same minute).
	upsertTestActivity(t, s, 3, userID, "2026-10-15T18:00")
	upsertTestActivity(t, s, 2, userID, "2026-10-15T07:00")
	upsertTestActivity(t, s, 1, userID, "2026-10-15T07:00")

	// Positive controls for the WHERE clause: each of these must be left out.
	upsertTestActivity(t, s, 10, userID, "2026-10-14T23:59")      // day before
	upsertTestActivity(t, s, 11, userID, "2026-10-16T00:00")      // day after
	upsertTestActivity(t, s, 12, otherUserID, "2026-10-15T12:00") // other user, same day

	got, err := s.ListDay(t.Context(), userID, "2026-10-15")
	if err != nil {
		t.Fatalf("ListDay: %v", err)
	}

	gotIDs := make([]int64, 0, len(got))
	for _, a := range got {
		gotIDs = append(gotIDs, a.StravaID)
	}
	if want := []int64{1, 2, 3}; !slices.Equal(want, gotIDs) {
		t.Errorf("ListDay IDs: got %v, wanted %v", gotIDs, want)
	}

	// Rows must be fully scanned, not only their IDs.
	if len(got) > 0 {
		testutil.AssertEqual(t, "UserID", userID, got[0].UserID)
		testutil.AssertEqual(t, "MovingTime", 45*time.Minute, got[0].MovingTime)
		testutil.AssertEqual(t, "Day", "2026-10-15", got[0].Day())
	}
}

func TestActivityStore_listDayEmpty(t *testing.T) {
	s, _, userID := newTestActivityStore(t)

	got, err := s.ListDay(t.Context(), userID, "2026-10-15")
	if err != nil {
		t.Fatalf("ListDay: %v", err)
	}
	// Empty and non-nil: a nil slice would encode as JSON null, not [].
	if got == nil || len(got) != 0 {
		t.Errorf("ListDay on an empty day: got %#v, wanted an empty non-nil slice", got)
	}
}

func TestActivityStore_setPoints(t *testing.T) {
	s, _, userID := newTestActivityStore(t)
	created := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return created }
	upsertTestActivity(t, s, 1, userID, "2026-10-15T07:00")
	upsertTestActivity(t, s, 2, userID, "2026-10-15T08:00")

	// A later clock: SetPoints must not move UpdatedAt, which tracks changes
	// from Strava, not our recomputes.
	s.now = func() time.Time { return created.Add(time.Hour) }
	if err := s.SetPoints(t.Context(), 1, 4); err != nil {
		t.Fatalf("SetPoints: %v", err)
	}

	got, err := s.Get(t.Context(), 1)
	if err != nil {
		t.Fatalf("Get 1: %v", err)
	}
	testutil.AssertEqual(t, "Points", 4, got.Points)
	testutil.AssertTime(t, "UpdatedAt", created, got.UpdatedAt)

	// Positive control for the WHERE clause: the other activity is untouched.
	other, err := s.Get(t.Context(), 2)
	if err != nil {
		t.Fatalf("Get 2: %v", err)
	}
	testutil.AssertEqual(t, "other activity Points", 0, other.Points)
}

func TestActivityStore_delete(t *testing.T) {
	s, _, userID := newTestActivityStore(t)
	upsertTestActivity(t, s, 1, userID, "2026-10-15T07:00")
	upsertTestActivity(t, s, 2, userID, "2026-10-15T08:00")

	if err := s.Delete(t.Context(), 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := s.Get(t.Context(), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete: got error %v, wanted ErrNotFound", err)
	}

	// Positive control for the WHERE clause: the other activity is kept.
	if _, err := s.Get(t.Context(), 2); err != nil {
		t.Errorf("Get other activity after Delete: %v", err)
	}

	// Idempotent: Strava can send a delete for an activity that is already
	// gone (a retried webhook) or that we never stored.
	if err := s.Delete(t.Context(), 1); err != nil {
		t.Errorf("second Delete: got error %v, wanted nil", err)
	}
	if err := s.Delete(t.Context(), 999); err != nil {
		t.Errorf("Delete of an unknown activity: got error %v, wanted nil", err)
	}
}

func TestActivityStore_withTx(t *testing.T) {
	s, db, userID := newTestActivityStore(t)

	// Rolled back: the plain store must not see the row.
	tx, err := db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin rollback tx: %v", err)
	}
	upsertTestActivity(t, s.WithTx(tx), 1, userID, "2026-10-15T07:00")
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if _, err := s.Get(t.Context(), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after rollback: got error %v, wanted ErrNotFound", err)
	}

	// Committed: the plain store sees the row. Positive control: without it,
	// a WithTx that writes nowhere would pass the rollback check above.
	tx, err = db.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatalf("begin commit tx: %v", err)
	}
	upsertTestActivity(t, s.WithTx(tx), 2, userID, "2026-10-15T08:00")
	if err := tx.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := s.Get(t.Context(), 2); err != nil {
		t.Errorf("Get after commit: %v", err)
	}
}

func TestActivityStore_deletingUserDeletesActivities(t *testing.T) {
	s, db, userID := newTestActivityStore(t)
	otherUserID := createTestUser(t, db, 43)
	upsertTestActivity(t, s, 1, userID, "2026-10-15T07:00")
	upsertTestActivity(t, s, 2, otherUserID, "2026-10-15T07:00")

	// There is no UserStore.Delete yet (#15), so delete the user directly.
	if _, err := db.ExecContext(t.Context(), "DELETE FROM users WHERE id = ?", userID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	if _, err := s.Get(t.Context(), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get activity of deleted user: got error %v, wanted ErrNotFound", err)
	}
	// Positive control: the other user's activity is kept.
	if _, err := s.Get(t.Context(), 2); err != nil {
		t.Errorf("Get activity of other user: %v", err)
	}
}

// newTestActivityStore returns an ActivityStore on a fresh database, the
// database itself (for direct SQL in tests), and the ID of a stored user that
// activities can reference.
func newTestActivityStore(t *testing.T) (*ActivityStore, *sql.DB, int64) {
	t.Helper()
	db := newTestDB(t)
	return NewActivityStore(db), db, createTestUser(t, db, 42)
}

// createTestUser stores a user with the given Strava ID and returns its
// internal ID.
func createTestUser(t *testing.T, db *sql.DB, stravaID int64) int64 {
	t.Helper()
	u, err := NewUserStore(db).Upsert(t.Context(), User{
		StravaID:     stravaID,
		FirstName:    "Test",
		LastName:     "Athlete",
		AccessToken:  "access",
		RefreshToken: "refresh",
	})
	if err != nil {
		t.Fatalf("create user %d: %v", stravaID, err)
	}
	return u.ID
}

// upsertTestActivity stores a counted run for userID starting at start
// (layout "2006-01-02T15:04") and fails the test on error.
func upsertTestActivity(t *testing.T, s *ActivityStore, stravaID, userID int64, start string) {
	t.Helper()
	startLocal, err := time.Parse("2006-01-02T15:04", start)
	if err != nil {
		t.Fatalf("parse start %q: %v", start, err)
	}
	_, err = s.Upsert(t.Context(), Activity{
		StravaID:   stravaID,
		UserID:     userID,
		SportType:  "Run",
		StartLocal: startLocal,
		MovingTime: 45 * time.Minute,
		Distance:   9000,
	})
	if err != nil {
		t.Fatalf("upsert activity %d: %v", stravaID, err)
	}
}
