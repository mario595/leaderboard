package activity_test

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/activity"
	"github.com/mario595/leaderboard/internal/store"
)

func TestService_saveCapsDailyPoints(t *testing.T) {
	db, userID := newTestDB(t)
	svc := activity.NewService(db)

	// Three plain runs on one day: the two earliest score, the third does not.
	save(t, svc, run(1, userID, "2026-10-15T07:00"))
	save(t, svc, run(2, userID, "2026-10-15T12:00"))
	save(t, svc, run(3, userID, "2026-10-15T18:00"))

	assertPoints(t, db, map[int64]int{1: 2, 2: 2, 3: 0})
}

func TestService_saveEditChangesTopTwo(t *testing.T) {
	db, userID := newTestDB(t)
	svc := activity.NewService(db)
	save(t, svc, run(1, userID, "2026-10-15T07:00"))
	save(t, svc, run(2, userID, "2026-10-15T12:00"))
	save(t, svc, run(3, userID, "2026-10-15T18:00"))

	// The athlete marks the latest run as a race on Strava: it now outranks
	// the 12:00 run, which loses its slot.
	race := run(3, userID, "2026-10-15T18:00")
	race.WorkoutType = 1
	save(t, svc, race)

	assertPoints(t, db, map[int64]int{1: 2, 2: 0, 3: 4})
}

func TestService_saveMovedToAnotherDayRecomputesBothDays(t *testing.T) {
	db, userID := newTestDB(t)
	svc := activity.NewService(db)
	save(t, svc, run(1, userID, "2026-10-15T07:00"))
	save(t, svc, run(2, userID, "2026-10-15T12:00"))
	save(t, svc, run(3, userID, "2026-10-15T18:00"))
	save(t, svc, run(4, userID, "2026-10-16T07:00"))
	save(t, svc, run(5, userID, "2026-10-16T12:00"))

	// The athlete fixes the date of activity 1: it moves to the 16th, at the
	// latest time there.
	save(t, svc, run(1, userID, "2026-10-16T18:00"))

	assertPoints(t, db, map[int64]int{
		// 15th: activity 3 gets the slot that activity 1 freed.
		2: 2, 3: 2,
		// 16th: activity 1 is the third run there and gets 0.
		4: 2, 5: 2, 1: 0,
	})
}

func TestService_deleteFreesSlot(t *testing.T) {
	db, userID := newTestDB(t)
	svc := activity.NewService(db)
	save(t, svc, run(1, userID, "2026-10-15T07:00"))
	save(t, svc, run(2, userID, "2026-10-15T12:00"))
	save(t, svc, run(3, userID, "2026-10-15T18:00"))

	if err := svc.Delete(t.Context(), 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Activity 3 gets the slot activity 1 freed.
	assertPoints(t, db, map[int64]int{2: 2, 3: 2})
	if _, err := store.NewActivityStore(db).Get(t.Context(), 1); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get deleted activity: got error %v, wanted ErrNotFound", err)
	}
}

func TestService_deleteUnknownActivity(t *testing.T) {
	db, _ := newTestDB(t)
	svc := activity.NewService(db)

	// Strava sends deletes for activities we never stored, and retries
	// webhooks: nothing to do is not an error.
	if err := svc.Delete(t.Context(), 999); err != nil {
		t.Errorf("Delete unknown activity: got error %v, wanted nil", err)
	}
}

func TestService_otherUsersAreNotAffected(t *testing.T) {
	db, userID := newTestDB(t)
	otherUserID := createUser(t, db, 43)
	svc := activity.NewService(db)

	// Two runs each, same day. If the recompute grouped by day only, the
	// four runs would share two slots.
	save(t, svc, run(1, userID, "2026-10-15T07:00"))
	save(t, svc, run(2, otherUserID, "2026-10-15T08:00"))
	save(t, svc, run(3, userID, "2026-10-15T12:00"))
	save(t, svc, run(4, otherUserID, "2026-10-15T13:00"))

	assertPoints(t, db, map[int64]int{1: 2, 2: 2, 3: 2, 4: 2})

	// Deleting one user's activity must not change the other user's points.
	if err := svc.Delete(t.Context(), 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	assertPoints(t, db, map[int64]int{2: 2, 3: 2, 4: 2})
}

func TestService_saveIsIdempotent(t *testing.T) {
	db, userID := newTestDB(t)
	svc := activity.NewService(db)
	save(t, svc, run(1, userID, "2026-10-15T07:00"))
	save(t, svc, run(2, userID, "2026-10-15T12:00"))
	save(t, svc, run(3, userID, "2026-10-15T18:00"))
	want := map[int64]int{1: 2, 2: 2, 3: 0}
	assertPoints(t, db, want)

	// A retried webhook or the reconcile job saves the same data again.
	save(t, svc, run(3, userID, "2026-10-15T18:00"))
	save(t, svc, run(1, userID, "2026-10-15T07:00"))
	assertPoints(t, db, want)
}

// newTestDB returns a migrated database in a temporary directory and the ID
// of a stored user. It duplicates the store package's own helper: sharing it
// through testutil would make testutil import store while store's tests
// import testutil, an import cycle.
func newTestDB(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := store.Migrate(t.Context(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db, createUser(t, db, 42)
}

// createUser stores a user with the given Strava ID and returns its internal ID.
func createUser(t *testing.T, db *sql.DB, stravaID int64) int64 {
	t.Helper()
	u, err := store.NewUserStore(db).Upsert(t.Context(), store.User{
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

// run returns a 45-minute run, worth 2 points before the daily cap, starting
// at start (layout "2006-01-02T15:04").
func run(stravaID, userID int64, start string) store.Activity {
	startLocal, err := time.Parse("2006-01-02T15:04", start)
	if err != nil {
		panic(err) // a typo in a test literal, not a runtime condition
	}
	return store.Activity{
		StravaID:   stravaID,
		UserID:     userID,
		SportType:  "Run",
		StartLocal: startLocal,
		MovingTime: 45 * time.Minute,
		Distance:   9000,
	}
}

func save(t *testing.T, svc *activity.Service, a store.Activity) {
	t.Helper()
	if err := svc.Save(t.Context(), a); err != nil {
		t.Fatalf("Save activity %d: %v", a.StravaID, err)
	}
}

// assertPoints reads each activity back and compares its stored points.
func assertPoints(t *testing.T, db *sql.DB, want map[int64]int) {
	t.Helper()
	activities := store.NewActivityStore(db)
	for id, wantPoints := range want {
		a, err := activities.Get(t.Context(), id)
		if err != nil {
			t.Errorf("Get activity %d: %v", id, err)
			continue
		}
		if a.Points != wantPoints {
			t.Errorf("activity %d: got %d points, wanted %d", id, a.Points, wantPoints)
		}
	}
}
