package store

import (
	"database/sql"
	"errors"
	"sync"
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

func TestLogin(t *testing.T) {
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
	if _, err := s.Upsert(t.Context(), User{
		StravaID: 2, FirstName: "Leia", LastName: "Organa",
		AccessToken: "leia-access", RefreshToken: "leia-refresh", TokenExpiresAt: t0,
	}); err != nil {
		t.Fatalf("upsert leia: %v", err)
	}

	// Strava sends the current name and new tokens on every login.
	s.now = func() time.Time { return t1 }
	newExpiry := t1.Add(6 * time.Hour)
	got, err := s.Login(t.Context(), User{
		StravaID: 1, FirstName: "Anakin", LastName: "Vader",
		AccessToken: "new-access", RefreshToken: "new-refresh", TokenExpiresAt: newExpiry,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	testutil.AssertEqual(t, "ID", luke.ID, got.ID)
	testutil.AssertEqual(t, "StravaID", int64(1), got.StravaID)
	testutil.AssertEqual(t, "FirstName", "Anakin", got.FirstName)
	testutil.AssertEqual(t, "LastName", "Vader", got.LastName)
	testutil.AssertEqual(t, "AccessToken", "new-access", got.AccessToken)
	testutil.AssertEqual(t, "RefreshToken", "new-refresh", got.RefreshToken)
	testutil.AssertTime(t, "TokenExpiresAt", newExpiry, got.TokenExpiresAt)
	testutil.AssertTime(t, "CreatedAt", t0, got.CreatedAt)
	testutil.AssertTime(t, "UpdatedAt", t1, got.UpdatedAt)

	// What Login returned is what was stored.
	stored, err := s.GetByStravaID(t.Context(), 1)
	if err != nil {
		t.Fatalf("GetByStravaID luke: %v", err)
	}
	testutil.AssertEqual(t, "stored FirstName", "Anakin", stored.FirstName)
	testutil.AssertEqual(t, "stored AccessToken", "new-access", stored.AccessToken)

	// Positive control for the WHERE clause: the other user is untouched.
	other, err := s.GetByStravaID(t.Context(), 2)
	if err != nil {
		t.Fatalf("GetByStravaID leia: %v", err)
	}
	testutil.AssertEqual(t, "other user FirstName", "Leia", other.FirstName)
	testutil.AssertEqual(t, "other user AccessToken", "leia-access", other.AccessToken)
}

func TestLogin_unknownAthlete(t *testing.T) {
	s := newTestStore(t)

	_, err := s.Login(t.Context(), User{StravaID: 1, FirstName: "Luke", AccessToken: "a", RefreshToken: "r"})

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Login unknown athlete: got error %v, wanted ErrNotFound", err)
	}
	// Login never creates a user: that needs an invitation.
	if _, err := s.GetByStravaID(t.Context(), 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetByStravaID after Login: got error %v, wanted ErrNotFound", err)
	}
}

func TestSignUp(t *testing.T) {
	s := newTestStore(t)
	invitations := NewInvitationStore(s.db)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	inv, err := invitations.Create(t.Context(), 2, time.Time{})
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	expiry := now.Add(6 * time.Hour)

	got, err := s.SignUp(t.Context(), User{
		StravaID: 1, FirstName: "Luke", LastName: "Skywalker",
		AccessToken: "luke-access", RefreshToken: "luke-refresh", TokenExpiresAt: expiry,
	}, inv.Code)
	if err != nil {
		t.Fatalf("SignUp: %v", err)
	}

	if got.ID == 0 {
		t.Error("ID is 0, want the stored row's ID")
	}
	testutil.AssertEqual(t, "StravaID", int64(1), got.StravaID)
	testutil.AssertEqual(t, "FirstName", "Luke", got.FirstName)
	testutil.AssertEqual(t, "LastName", "Skywalker", got.LastName)
	testutil.AssertEqual(t, "AccessToken", "luke-access", got.AccessToken)
	testutil.AssertEqual(t, "RefreshToken", "luke-refresh", got.RefreshToken)
	testutil.AssertTime(t, "TokenExpiresAt", expiry, got.TokenExpiresAt)
	testutil.AssertTime(t, "CreatedAt", now, got.CreatedAt)
	testutil.AssertTime(t, "UpdatedAt", now, got.UpdatedAt)

	if _, err := s.GetByStravaID(t.Context(), 1); err != nil {
		t.Errorf("GetByStravaID after SignUp: %v", err)
	}
	testutil.AssertEqual(t, "invitation uses", 1, findInvitation(t, invitations, inv.Code).Uses)
	testutil.AssertEqual(t, "users.invitation_id", inv.ID, userInvitationID(t, s.db, got.ID))
}

// Every reason an invitation cannot be used gives the same error, creates no
// user and takes no seat.
func TestSignUp_invalidInvitation(t *testing.T) {
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	expires := created.Add(24 * time.Hour)

	tests := []struct {
		name string
		// setup returns the code to sign up with, and when.
		setup func(t *testing.T, s *UserStore, inv *InvitationStore) (code string, at time.Time)
	}{
		{"empty code", func(t *testing.T, s *UserStore, inv *InvitationStore) (string, time.Time) {
			createTestInvitation(t, inv, 1, time.Time{})
			return "", created
		}},
		{"unknown code", func(t *testing.T, s *UserStore, inv *InvitationStore) (string, time.Time) {
			createTestInvitation(t, inv, 1, time.Time{})
			return "NOSUCHCODE", created
		}},
		{"used up", func(t *testing.T, s *UserStore, inv *InvitationStore) (string, time.Time) {
			code := createTestInvitation(t, inv, 1, time.Time{}).Code
			if _, err := s.SignUp(t.Context(), User{StravaID: 2, AccessToken: "a", RefreshToken: "r"}, code); err != nil {
				t.Fatalf("first SignUp: %v", err)
			}
			return code, created
		}},
		{"revoked", func(t *testing.T, s *UserStore, inv *InvitationStore) (string, time.Time) {
			code := createTestInvitation(t, inv, 1, time.Time{}).Code
			if err := inv.Revoke(t.Context(), code); err != nil {
				t.Fatalf("Revoke: %v", err)
			}
			return code, created
		}},
		{"expires at that second", func(t *testing.T, s *UserStore, inv *InvitationStore) (string, time.Time) {
			return createTestInvitation(t, inv, 1, expires).Code, expires
		}},
		{"expired", func(t *testing.T, s *UserStore, inv *InvitationStore) (string, time.Time) {
			return createTestInvitation(t, inv, 1, expires).Code, expires.Add(time.Hour)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestStore(t)
			invitations := NewInvitationStore(s.db)
			s.now = func() time.Time { return created }
			invitations.now = func() time.Time { return created }
			code, at := tt.setup(t, s, invitations)
			usesBefore := totalUses(t, invitations)
			s.now = func() time.Time { return at }

			_, err := s.SignUp(t.Context(), User{StravaID: 1, AccessToken: "a", RefreshToken: "r"}, code)

			if !errors.Is(err, ErrInvalidInvitation) {
				t.Errorf("SignUp: got error %v, wanted ErrInvalidInvitation", err)
			}
			if _, err := s.GetByStravaID(t.Context(), 1); !errors.Is(err, ErrNotFound) {
				t.Errorf("GetByStravaID after a rejected SignUp: got error %v, wanted ErrNotFound", err)
			}
			testutil.AssertEqual(t, "invitation uses", usesBefore, totalUses(t, invitations))
		})
	}
}

// Positive control for the expiry cases above: one second before it expires,
// the invitation still works.
func TestSignUp_justBeforeExpiry(t *testing.T) {
	s := newTestStore(t)
	invitations := NewInvitationStore(s.db)
	created := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	invitations.now = func() time.Time { return created }
	expires := created.Add(24 * time.Hour)
	code := createTestInvitation(t, invitations, 1, expires).Code
	s.now = func() time.Time { return expires.Add(-time.Second) }

	if _, err := s.SignUp(t.Context(), User{StravaID: 1, AccessToken: "a", RefreshToken: "r"}, code); err != nil {
		t.Errorf("SignUp one second before expiry: %v", err)
	}
}

// An athlete who already has an account fails the INSERT. The transaction must
// give the seat back.
func TestSignUp_existingAthleteKeepsTheSeat(t *testing.T) {
	s := newTestStore(t)
	invitations := NewInvitationStore(s.db)
	if _, err := s.Upsert(t.Context(), User{StravaID: 1, FirstName: "Luke", AccessToken: "a", RefreshToken: "r"}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	code := createTestInvitation(t, invitations, 1, time.Time{}).Code

	_, err := s.SignUp(t.Context(), User{StravaID: 1, FirstName: "Anakin", AccessToken: "a", RefreshToken: "r"}, code)

	if err == nil {
		t.Fatal("SignUp of an existing athlete: got no error")
	}
	if errors.Is(err, ErrInvalidInvitation) {
		t.Errorf("SignUp of an existing athlete: got %v, the invitation is not the problem", err)
	}
	testutil.AssertEqual(t, "invitation uses", 0, findInvitation(t, invitations, code).Uses)
	stored, err := s.GetByStravaID(t.Context(), 1)
	if err != nil {
		t.Fatalf("GetByStravaID: %v", err)
	}
	testutil.AssertEqual(t, "FirstName", "Luke", stored.FirstName)
}

// Ten athletes race for the last seat: exactly one gets it. Run with -race.
func TestSignUp_concurrentLastSeat(t *testing.T) {
	s := newTestStore(t)
	invitations := NewInvitationStore(s.db)
	code := createTestInvitation(t, invitations, 1, time.Time{}).Code

	const athletes = 10
	errs := make([]error, athletes)
	var wg sync.WaitGroup
	for i := range athletes {
		wg.Go(func() {
			_, errs[i] = s.SignUp(t.Context(), User{StravaID: int64(i + 1), AccessToken: "a", RefreshToken: "r"}, code)
		})
	}
	wg.Wait()

	signedUp := 0
	for i, err := range errs {
		switch {
		case err == nil:
			signedUp++
		case errors.Is(err, ErrInvalidInvitation):
		default:
			t.Errorf("athlete %d: unexpected error %v", i+1, err)
		}
	}
	testutil.AssertEqual(t, "athletes signed up", 1, signedUp)
	testutil.AssertEqual(t, "invitation uses", 1, findInvitation(t, invitations, code).Uses)
	var users int
	if err := s.db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM users").Scan(&users); err != nil {
		t.Fatalf("count users: %v", err)
	}
	testutil.AssertEqual(t, "users", 1, users)
}

func createTestInvitation(t *testing.T, s *InvitationStore, maxUses int, expiresAt time.Time) Invitation {
	t.Helper()
	inv, err := s.Create(t.Context(), maxUses, expiresAt)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	return inv
}

// totalUses sums the uses of every invitation.
func totalUses(t *testing.T, s *InvitationStore) int {
	t.Helper()
	list, err := s.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	total := 0
	for _, inv := range list {
		total += inv.Uses
	}
	return total
}

// userInvitationID reads users.invitation_id, which User does not expose.
func userInvitationID(t *testing.T, db *sql.DB, userID int64) int64 {
	t.Helper()
	var id sql.NullInt64
	if err := db.QueryRowContext(t.Context(), "SELECT invitation_id FROM users WHERE id = ?", userID).Scan(&id); err != nil {
		t.Fatalf("read invitation_id: %v", err)
	}
	if !id.Valid {
		t.Fatalf("user %d has no invitation_id", userID)
	}
	return id.Int64
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

func TestList(t *testing.T) {
	s := newTestStore(t)

	empty, err := s.List(t.Context())
	if err != nil {
		t.Fatalf("List on an empty table: %v", err)
	}
	testutil.AssertEqual(t, "users in an empty table", 0, len(empty))

	expiry := time.Date(2026, 10, 15, 18, 0, 0, 0, time.UTC)
	// Strava IDs out of order: List is ordered by our ID, not theirs.
	for _, u := range []User{
		{StravaID: 3, FirstName: "Luke", AccessToken: "luke-access", RefreshToken: "luke-refresh", TokenExpiresAt: expiry},
		{StravaID: 1, FirstName: "Leia", AccessToken: "leia-access", RefreshToken: "leia-refresh", TokenExpiresAt: expiry},
		{StravaID: 2, FirstName: "Han", AccessToken: "han-access", RefreshToken: "han-refresh", TokenExpiresAt: expiry},
	} {
		if _, err := s.Upsert(t.Context(), u); err != nil {
			t.Fatalf("upsert %s: %v", u.FirstName, err)
		}
	}

	users, err := s.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(users) != 3 {
		t.Fatalf("List returned %d users, want 3", len(users))
	}
	for i, want := range []string{"Luke", "Leia", "Han"} {
		testutil.AssertEqual(t, "user in ID order", want, users[i].FirstName)
		if i > 0 && users[i].ID <= users[i-1].ID {
			t.Errorf("users not in ID order: %d after %d", users[i].ID, users[i-1].ID)
		}
	}
	// Full users: the reconcile needs the tokens.
	testutil.AssertEqual(t, "StravaID", int64(3), users[0].StravaID)
	testutil.AssertEqual(t, "AccessToken", "luke-access", users[0].AccessToken)
	testutil.AssertEqual(t, "RefreshToken", "luke-refresh", users[0].RefreshToken)
	testutil.AssertTime(t, "TokenExpiresAt", expiry, users[0].TokenExpiresAt)
}
