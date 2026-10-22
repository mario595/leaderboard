package store

import (
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

func newTestStore(t *testing.T) *UserStore {
	t.Helper()
	return NewUserStore(newTestDB(t))
}
