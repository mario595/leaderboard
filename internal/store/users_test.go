package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
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

	if result.ID == 0 {
		t.Errorf("ID should not be 0")
	}

	assertEqual(t, "StravaID", u.StravaID, result.StravaID)
	assertEqual(t, "FirstName", u.FirstName, result.FirstName)
	assertEqual(t, "LastName", u.LastName, result.LastName)
	assertEqual(t, "AccessToken", u.AccessToken, result.AccessToken)
	assertEqual(t, "RefreshToken", u.RefreshToken, result.RefreshToken)
	assertTime(t, "TokenExpiresAt", u.TokenExpiresAt, result.TokenExpiresAt)
	assertTime(t, "CreatedAt", fixed, result.CreatedAt)
	assertTime(t, "UpdatedAt", fixed, result.UpdatedAt)
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

	assertEqual(t, "ID", insertedUser.ID, result.ID)
	assertEqual(t, "FirstName", "Anakin", result.FirstName)
	assertEqual(t, "AccessToken", "secondAccessToken123", result.AccessToken)
	assertTime(t, "CreatedAt", t0, result.CreatedAt)
	assertTime(t, "UpdatedAt", result.UpdatedAt, t1)
}

func TestNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetByStravaID(t.Context(), 123)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Unexpected error. Got %v, expected ErrNotFound", err)
	}
}

func newTestStore(t *testing.T) *UserStore {
	t.Helper()
	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	err = Migrate(t.Context(), db)
	if err != nil {
		t.Fatalf("Migrate db: %v", err)
	}

	return NewUserStore(db)
}

func assertEqual[T comparable](t *testing.T, name string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("unexpected value %s: wanted %v, got %v", name, want, got)
	}
}

func assertTime(t *testing.T, propertyName string, wanted, got time.Time) {
	t.Helper()
	if !wanted.Truncate(time.Second).Equal(got) {
		t.Errorf("unexpected value time %s: wanted %v, got %v", propertyName, wanted, got)
	}
}
