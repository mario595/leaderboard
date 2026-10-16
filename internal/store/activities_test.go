package store

import (
	"database/sql"
	"errors"
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

// newTestActivityStore returns an ActivityStore on a fresh database, the
// database itself (for direct SQL in tests), and the ID of a stored user that
// activities can reference.
func newTestActivityStore(t *testing.T) (*ActivityStore, *sql.DB, int64) {
	t.Helper()
	db := newTestDB(t)

	u, err := NewUserStore(db).Upsert(t.Context(), User{
		StravaID:     42,
		FirstName:    "Test",
		LastName:     "Athlete",
		AccessToken:  "access",
		RefreshToken: "refresh",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	return NewActivityStore(db), db, u.ID
}
