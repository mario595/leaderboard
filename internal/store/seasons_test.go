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
}

// TestSeasonStore_overlap checks every shape of overlap against an existing
// season from 2026-09-01 to 2026-09-30. Both bounds are inclusive, so seasons
// that share a single day overlap, and seasons on adjacent days do not.
func TestSeasonStore_overlap(t *testing.T) {
	testCases := []struct {
		name        string
		start       string
		end         string
		wantOverlap bool
	}{
		{name: "inside the existing season", start: "2026-09-10", end: "2026-09-20", wantOverlap: true},
		{name: "straddles the start", start: "2026-08-15", end: "2026-09-05", wantOverlap: true},
		{name: "straddles the end", start: "2026-09-25", end: "2026-10-10", wantOverlap: true},
		{name: "contains the existing season", start: "2026-08-01", end: "2026-10-31", wantOverlap: true},
		{name: "identical dates", start: "2026-09-01", end: "2026-09-30", wantOverlap: true},
		{name: "shares only the last day", start: "2026-09-30", end: "2026-10-15", wantOverlap: true},
		{name: "shares only the first day", start: "2026-08-15", end: "2026-09-01", wantOverlap: true},
		{name: "adjacent day after", start: "2026-10-01", end: "2026-10-31", wantOverlap: false},
		{name: "adjacent day before", start: "2026-08-01", end: "2026-08-31", wantOverlap: false},
		{name: "far before", start: "2026-01-01", end: "2026-02-01", wantOverlap: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// A fresh store for every case, so cases cannot affect each other.
			st := newTestSeasonStore(t)

			existing, err := st.Create(t.Context(), Season{
				Name:      "Existing",
				StartDate: date(t, "2026-09-01"),
				EndDate:   date(t, "2026-09-30"),
			})
			if err != nil {
				t.Fatalf("create existing season: %v", err)
			}

			// A different name, so a failure can only come from the overlap check
			// and not from the UNIQUE constraint on the name.
			_, err = st.Create(t.Context(), Season{
				Name:      "Candidate",
				StartDate: date(t, tc.start),
				EndDate:   date(t, tc.end),
			})

			wantSeasons := 2
			if tc.wantOverlap {
				wantSeasons = 1

				var oe *OverlapError
				if !errors.As(err, &oe) {
					t.Fatalf("Create error = %v, want *OverlapError", err)
				}
				assertEqual(t, "OverlapError.With.ID", existing.ID, oe.With.ID)
			} else if err != nil {
				t.Fatalf("Create error = %v, want no error", err)
			}

			// A rejected season must leave nothing behind.
			seasons, err := st.List(t.Context())
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			assertEqual(t, "number of seasons", wantSeasons, len(seasons))
		})
	}
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
