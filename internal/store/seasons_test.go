package store

import (
	"errors"
	"testing"
	"time"
)

func TestSeasonStore_createAndList(t *testing.T) {
	st := newTestSeasonStore(t)
	fixed := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return fixed }

	// List should return empty list if no seasons are stored
	seasons, err := st.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(seasons) != 0 {
		t.Errorf("Unexpected length: %d, expected 0", len(seasons))
	}

	season := Season{
		Name:      "Test Season",
		StartDate: date(t, "2026-11-13"),
		EndDate:   date(t, "2026-11-15"),
	}

	// Create a new Season
	s, err := st.Create(t.Context(), season)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.ID == 0 {
		t.Errorf("Unexpected empty ID")
	}
	assertEqual(t, "Name", season.Name, s.Name)
	assertTime(t, "CreatedAt", fixed, s.CreatedAt)
	assertTime(t, "StartDate", season.StartDate, s.StartDate)
	assertTime(t, "EndDate", season.EndDate, s.EndDate)

	// New season is listed
	seasons, err = st.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(seasons) != 1 {
		t.Fatalf("Unexpected length: %d, expected 1", len(seasons))
	}
	assertEqual(t, "seasons[0].ID", s.ID, seasons[0].ID)

	// Overlapping seasons fail
	season.Name = "Second Test Season"
	season.StartDate = date(t, "2026-11-14")
	_, err = st.Create(t.Context(), season)
	if err == nil {
		t.Fatalf("Create: Expected error")
	}
	var oe *OverlapError
	if !errors.As(err, &oe) {
		t.Fatalf("Unexpected error, expected OverlapError, got %v", err)
	}

	if oe.With.ID != s.ID {
		t.Errorf("Unexpected With in OverlapError. Expected %d, got %d", s.ID, oe.With.ID)
	}

	// Another overlap
	season.Name = "Second Test Season"
	season.StartDate = date(t, "2026-11-15")
	season.EndDate = date(t, "2026-11-17")
	_, err = st.Create(t.Context(), season)
	if err == nil {
		t.Fatalf("Create: Expected error")
	}
	if !errors.As(err, &oe) {
		t.Fatalf("Unexpected error, expected OverlapError, got %v", err)
	}

	if oe.With.ID != s.ID {
		t.Errorf("Unexpected With in OverlapError. Expected %d, got %d", s.ID, oe.With.ID)
	}

	// Another overlap
	season.Name = "Second Test Season"
	season.StartDate = date(t, "2026-11-11")
	season.EndDate = date(t, "2026-11-13")
	_, err = st.Create(t.Context(), season)
	if err == nil {
		t.Fatalf("Create: Expected error")
	}
	if !errors.As(err, &oe) {
		t.Fatalf("Unexpected error, expected OverlapError, got %v", err)
	}

	if oe.With.ID != s.ID {
		t.Errorf("Unexpected With in OverlapError. Expected %d, got %d", s.ID, oe.With.ID)
	}

	// List still holds only one season
	seasons, err = st.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(seasons) != 1 {
		t.Fatalf("Unexpected length: %d, expected 1", len(seasons))
	}
	assertEqual(t, "seasons[0].ID", s.ID, seasons[0].ID)

}

func newTestSeasonStore(t *testing.T) *SeasonStore {
	t.Helper()
	return NewSeasonStore(newTestDB(t))
}

func date(t *testing.T, dateString string) time.Time {
	t.Helper()
	parsedTime, err := ParseDate(dateString)
	if err != nil {
		t.Fatalf("parseTime: %v", err)
	}
	return parsedTime
}
