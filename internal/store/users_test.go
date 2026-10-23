package store

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestRoundtrip(t *testing.T) {
	s := newTestStore(t)
	fixed := time.Now()
	s.now = func() time.Time { return fixed }
	u := User{
		StravaID:       int64(999),
		FirstName:      "Luke",
		LastName:       "Skywalker",
		AccessToken:    "accessToken123",
		RefreshToken:   "refreshToken123",
		TokenExpiresAt: time.Now().Add(time.Hour),
	}

	_, err := s.Upsert(t.Context(), u)
	if err != nil {
		t.Fatalf("error upsert: %v", err)
	}

	result, err := s.GetByStravaID(t.Context(), 999)
	if err != nil {
		t.Fatalf("testRoundtrip getByStravaID: %v", err)
	}

	id := result.ID

	if id == 0 {
		t.Errorf("ID should not be 0")
	}

	testutil.AssertEqual(t, "StravaID", u.StravaID, result.StravaID)
	testutil.AssertEqual(t, "FirstName", u.FirstName, result.FirstName)
	testutil.AssertEqual(t, "LastName", u.LastName, result.LastName)
	testutil.AssertEqual(t, "AccessToken", u.AccessToken, result.AccessToken)
	testutil.AssertEqual(t, "RefreshToken", u.RefreshToken, result.RefreshToken)
	testutil.AssertTime(t, "TokenExpiresAt", u.TokenExpiresAt, result.TokenExpiresAt)
	testutil.AssertTime(t, "CreatedAt", fixed, result.CreatedAt)
	testutil.AssertTime(t, "UpdatedAt", fixed, result.UpdatedAt)

	result, err = s.GetByID(t.Context(), id)
	if err != nil {
		t.Fatalf("testRoundtrip getByID: %v", err)
	}

	if result.ID != id {
		t.Errorf("ID should be %d", id)
	}

	testutil.AssertEqual(t, "StravaID", u.StravaID, result.StravaID)
	testutil.AssertEqual(t, "FirstName", u.FirstName, result.FirstName)
	testutil.AssertEqual(t, "LastName", u.LastName, result.LastName)
	testutil.AssertEqual(t, "AccessToken", u.AccessToken, result.AccessToken)
	testutil.AssertEqual(t, "RefreshToken", u.RefreshToken, result.RefreshToken)
	testutil.AssertTime(t, "TokenExpiresAt", u.TokenExpiresAt, result.TokenExpiresAt)
	testutil.AssertTime(t, "CreatedAt", fixed, result.CreatedAt)
	testutil.AssertTime(t, "UpdatedAt", fixed, result.UpdatedAt)
}

func TestUpsertUpdatesNotDuplicates(t *testing.T) {
	s := newTestStore(t)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t0 }
	u := User{
		StravaID:       int64(999),
		FirstName:      "Luke",
		LastName:       "Skywalker",
		AccessToken:    "accessToken123",
		RefreshToken:   "refreshToken123",
		TokenExpiresAt: time.Now().Add(time.Hour),
	}

	insertedUser, err := s.Upsert(t.Context(), u)
	if err != nil {
		t.Fatalf("error upsert: %v", err)
	}
	t1 := time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return t1 }
	u.AccessToken = "secondAccessToken123"
	u.FirstName = "Anakin"
	_, err = s.Upsert(t.Context(), u)
	if err != nil {
		t.Fatalf("error upsert: %v", err)
	}

	result, err := s.GetByStravaID(t.Context(), 999)
	if err != nil {
		t.Fatalf("testRoundtrip getByStravaID: %v", err)
	}

	testutil.AssertEqual(t, "ID", insertedUser.ID, result.ID)
	testutil.AssertEqual(t, "FirstName", "Anakin", result.FirstName)
	testutil.AssertEqual(t, "AccessToken", "secondAccessToken123", result.AccessToken)
	testutil.AssertTime(t, "CreatedAt", t0, result.CreatedAt)
	testutil.AssertTime(t, "UpdatedAt", result.UpdatedAt, t1)
}

func TestGetByStravaID_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetByStravaID(t.Context(), 123)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Unexpected error. Got %v, expected ErrNotFound", err)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetByID(t.Context(), 123)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Unexpected error. Got %v, expected ErrNotFound", err)
	}
}

func TestUpdateTokens(t *testing.T) {
	s := newTestStore(t)
	t0 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	s.now = func() time.Time { return t0 }

	luke, err := s.Upsert(t.Context(), User{
		StravaID: 1, FirstName: "Luke", LastName: "Skywalker",
		AccessToken: "luke-access", RefreshToken: "luke-refresh", TokenExpiresAt: t0,
	})
	if err != nil {
		t.Fatalf("upsert luke: %v", err)
	}
	leia, err := s.Upsert(t.Context(), User{
		StravaID: 2, FirstName: "Leia", LastName: "Organa",
		AccessToken: "leia-access", RefreshToken: "leia-refresh", TokenExpiresAt: t0,
	})
	if err != nil {
		t.Fatalf("upsert leia: %v", err)
	}

	s.now = func() time.Time { return t1 }
	newExpiry := t1.Add(6 * time.Hour)
	if err := s.UpdateTokens(t.Context(), luke.ID, "new-access", "new-refresh", newExpiry); err != nil {
		t.Fatalf("UpdateTokens: %v", err)
	}

	got, err := s.GetByID(t.Context(), luke.ID)
	if err != nil {
		t.Fatalf("GetByID luke: %v", err)
	}
	testutil.AssertEqual(t, "AccessToken", "new-access", got.AccessToken)
	testutil.AssertEqual(t, "RefreshToken", "new-refresh", got.RefreshToken)
	testutil.AssertTime(t, "TokenExpiresAt", newExpiry, got.TokenExpiresAt)
	testutil.AssertTime(t, "UpdatedAt", t1, got.UpdatedAt)
	testutil.AssertTime(t, "CreatedAt", t0, got.CreatedAt)
	testutil.AssertEqual(t, "FirstName", "Luke", got.FirstName)

	// Positive control for the WHERE clause: the other user keeps her tokens.
	other, err := s.GetByID(t.Context(), leia.ID)
	if err != nil {
		t.Fatalf("GetByID leia: %v", err)
	}
	testutil.AssertEqual(t, "other user AccessToken", "leia-access", other.AccessToken)
	testutil.AssertEqual(t, "other user RefreshToken", "leia-refresh", other.RefreshToken)
}

func TestUpdateTokens_NotFound(t *testing.T) {
	s := newTestStore(t)

	// A silent no-op would lose a rotated refresh token: unknown users are an error.
	err := s.UpdateTokens(t.Context(), 123, "access", "refresh", time.Now())
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("UpdateTokens unknown user: got error %v, wanted ErrNotFound", err)
	}
}

func TestDeleteByStravaID(t *testing.T) {
	db := newTestDB(t)
	users := NewUserStore(db)
	activities := NewActivityStore(db)
	events := NewEventStore(db)

	// Two athletes with an activity and an event each.
	const leaving, staying = int64(1001), int64(1002)
	leavingID := createTestUser(t, db, leaving)
	stayingID := createTestUser(t, db, staying)
	upsertTestActivity(t, activities, 1, leavingID, "2026-10-15T07:00")
	upsertTestActivity(t, activities, 2, stayingID, "2026-10-15T08:00")
	for _, owner := range []int64{leaving, staying} {
		if _, err := events.Insert(t.Context(), Event{
			ObjectType: "activity", ObjectID: owner * 10, AspectType: "create",
			OwnerID: owner, EventTime: time.Unix(1760521800, 0), Updates: "{}",
		}); err != nil {
			t.Fatalf("insert event for %d: %v", owner, err)
		}
	}

	deleted, err := users.DeleteByStravaID(t.Context(), leaving)
	if err != nil {
		t.Fatalf("DeleteByStravaID: %v", err)
	}
	testutil.AssertEqual(t, "deleted", true, deleted)

	// Everything of the leaving athlete is gone. The activity goes through
	// ON DELETE CASCADE, which only works with the foreign_keys pragma on.
	if _, err := users.GetByStravaID(t.Context(), leaving); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByStravaID after delete: got error %v, wanted ErrNotFound", err)
	}
	testutil.AssertEqual(t, "leaving athlete's activities", 0, countRows(t, db, "activities", "user_id", leavingID))
	testutil.AssertEqual(t, "leaving athlete's events", 0, countRows(t, db, "webhook_events", "owner_id", leaving))

	// Positive control: the other athlete keeps everything.
	if _, err := users.GetByStravaID(t.Context(), staying); err != nil {
		t.Errorf("GetByStravaID for the other athlete: %v", err)
	}
	testutil.AssertEqual(t, "other athlete's activities", 1, countRows(t, db, "activities", "user_id", stayingID))
	testutil.AssertEqual(t, "other athlete's events", 1, countRows(t, db, "webhook_events", "owner_id", staying))

	// Repeated (Strava retry) or never a user: nothing to delete, no error.
	deleted, err = users.DeleteByStravaID(t.Context(), leaving)
	if err != nil {
		t.Fatalf("second DeleteByStravaID: %v", err)
	}
	testutil.AssertEqual(t, "deleted twice", false, deleted)
	deleted, err = users.DeleteByStravaID(t.Context(), 999)
	if err != nil {
		t.Fatalf("DeleteByStravaID for an unknown athlete: %v", err)
	}
	testutil.AssertEqual(t, "deleted unknown", false, deleted)

	// Events of a non-user (the webhook stores them, #13) are removed too,
	// but no user was deleted: deleted must say so.
	const neverUser = int64(1003)
	if _, err := events.Insert(t.Context(), Event{
		ObjectType: "activity", ObjectID: 30, AspectType: "create",
		OwnerID: neverUser, EventTime: time.Unix(1760521800, 0), Updates: "{}",
	}); err != nil {
		t.Fatalf("insert event for a non-user: %v", err)
	}
	deleted, err = users.DeleteByStravaID(t.Context(), neverUser)
	if err != nil {
		t.Fatalf("DeleteByStravaID for a non-user with events: %v", err)
	}
	testutil.AssertEqual(t, "deleted non-user with events", false, deleted)
	testutil.AssertEqual(t, "non-user's events", 0, countRows(t, db, "webhook_events", "owner_id", neverUser))
}

// countRows counts the rows of table whose column equals value. table and
// column come from the test, never from input.
func countRows(t *testing.T, db *sql.DB, table, column string, value int64) int {
	t.Helper()
	var n int
	query := "SELECT COUNT(*) FROM " + table + " WHERE " + column + " = ?"
	if err := db.QueryRowContext(t.Context(), query, value).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func newTestStore(t *testing.T) *UserStore {
	t.Helper()
	return NewUserStore(newTestDB(t))
}
