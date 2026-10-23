package store

import (
	"database/sql"
	"slices"
	"testing"
)

func TestActivityStore_standings(t *testing.T) {
	db := newTestDB(t)
	s := NewActivityStore(db)

	// Created out of alphabetical order, so user IDs do not follow the names:
	// for tied points, only the ORDER BY on names gives the expected order.
	linus := createNamedUser(t, db, 5, "Linus", "Torvalds")
	barbara := createNamedUser(t, db, 3, "Barbara", "Liskov")
	ada := createNamedUser(t, db, 1, "Ada", "Lovelace")
	grace := createNamedUser(t, db, 4, "Grace", "Hopper")
	alan := createNamedUser(t, db, 2, "Alan", "Turing")

	// Ada: two activities in the season (4 + 2), and two just outside it.
	addScoredActivity(t, s, 101, ada, "2026-10-15T07:00", 4)
	addScoredActivity(t, s, 102, ada, "2026-11-02T18:00", 2)
	addScoredActivity(t, s, 103, ada, "2026-08-31T23:59", 4) // day before the start
	addScoredActivity(t, s, 104, ada, "2027-01-01T00:00", 4) // day after the end
	// Alan: on the first and on the last day (both bounds inclusive).
	addScoredActivity(t, s, 201, alan, "2026-09-01T00:00", 2)
	addScoredActivity(t, s, 202, alan, "2026-12-31T23:59", 2)
	// Barbara: same total as Alan, so the name decides the order.
	addScoredActivity(t, s, 301, barbara, "2026-10-01T07:00", 4)
	// Grace: activities, but none in the season. With the date filter in
	// WHERE instead of ON she would disappear from the result.
	addScoredActivity(t, s, 401, grace, "2026-07-01T07:00", 4)
	// Linus: no activities at all.

	// Season: 2026-09-01 to 2026-12-31, both inclusive.
	got, err := s.Standings(t.Context(), date(t, "2026-09-01"), date(t, "2026-12-31"))
	if err != nil {
		t.Fatalf("Standings: %v", err)
	}

	want := []Standing{
		{UserID: ada, FirstName: "Ada", LastName: "Lovelace", Points: 6},
		{UserID: alan, FirstName: "Alan", LastName: "Turing", Points: 4},
		{UserID: barbara, FirstName: "Barbara", LastName: "Liskov", Points: 4},
		{UserID: grace, FirstName: "Grace", LastName: "Hopper", Points: 0},
		{UserID: linus, FirstName: "Linus", LastName: "Torvalds", Points: 0},
	}
	if !slices.Equal(want, got) {
		t.Errorf("Standings:\n got  %+v\n want %+v", got, want)
	}
}

func TestActivityStore_standingsWithoutUsers(t *testing.T) {
	s := NewActivityStore(newTestDB(t))

	got, err := s.Standings(t.Context(), date(t, "2026-09-01"), date(t, "2026-12-31"))
	if err != nil {
		t.Fatalf("Standings: %v", err)
	}
	// Empty and non-nil: the API encodes it as [] rather than null.
	if got == nil || len(got) != 0 {
		t.Errorf("Standings with no users: got %#v, wanted an empty non-nil slice", got)
	}
}

func createNamedUser(t *testing.T, db *sql.DB, stravaID int64, first, last string) int64 {
	t.Helper()
	u, err := NewUserStore(db).Upsert(t.Context(), User{
		StravaID: stravaID, FirstName: first, LastName: last,
		AccessToken: "access", RefreshToken: "refresh",
	})
	if err != nil {
		t.Fatalf("create user %s: %v", first, err)
	}
	return u.ID
}

// addScoredActivity stores an activity and sets its points directly: points
// normally come from the per-day recompute, which is not under test here.
func addScoredActivity(t *testing.T, s *ActivityStore, stravaID, userID int64, start string, points int) {
	t.Helper()
	upsertTestActivity(t, s, stravaID, userID, start)
	if err := s.SetPoints(t.Context(), stravaID, points); err != nil {
		t.Fatalf("set points of %d: %v", stravaID, err)
	}
}
