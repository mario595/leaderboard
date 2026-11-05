package main

import (
	"bytes"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

func TestBackfillCommand(t *testing.T) {
	db := newTestDB(t)
	autumn := createCLITestSeason(t, db)
	for _, stravaID := range []int64{1001, 1002} {
		if _, err := store.NewUserStore(db).Upsert(t.Context(), store.User{StravaID: stravaID, AccessToken: "a", RefreshToken: "r"}); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}

	out, err := runBackfillCmd(t, db, "-season", "1")
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}

	if want := `queued 2 backfill jobs for season #1 "Autumn 2026"` + "\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
	testutil.AssertEqual(t, "jobs stored", 2, countBackfillJobs(t, db, autumn.ID))

	// Twice in a row: both jobs are still waiting, nothing new is queued.
	out, err = runBackfillCmd(t, db, "-season", "1")
	if err != nil {
		t.Fatalf("second backfill: %v", err)
	}
	if want := `queued 0 backfill jobs for season #1 "Autumn 2026"` + "\n"; out != want {
		t.Errorf("second output = %q, want %q", out, want)
	}
	testutil.AssertEqual(t, "jobs stored after the second run", 2, countBackfillJobs(t, db, autumn.ID))
}

func TestBackfillCommand_unknownSeason(t *testing.T) {
	db := newTestDB(t)
	createCLITestSeason(t, db)

	_, err := runBackfillCmd(t, db, "-season", "2")

	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("backfill -season 2: error = %v, want store.ErrNotFound", err)
	}
}

func TestBackfillCommand_errors(t *testing.T) {
	testCases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no season", nil, "-season"},
		{"season 0", []string{"-season", "0"}, "-season"},
		{"not a number", []string{"-season", "autumn"}, "-season"},
		{"unknown flag", []string{"-bogus"}, "-bogus"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			createCLITestSeason(t, db)

			_, err := runBackfillCmd(t, db, tc.args...)
			if err == nil {
				t.Fatalf("backfillCommand(%q) succeeded, want an error containing %q", tc.args, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func createCLITestSeason(t *testing.T, db *sql.DB) store.Season {
	t.Helper()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	s, err := store.NewSeasonStore(db).Create(t.Context(), store.Season{Name: "Autumn 2026", StartDate: start, EndDate: end})
	if err != nil {
		t.Fatalf("create season: %v", err)
	}
	return s
}

func countBackfillJobs(t *testing.T, db *sql.DB, seasonID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT COUNT(*) FROM backfill_jobs WHERE season_id = ?", seasonID).Scan(&n); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	return n
}

func runBackfillCmd(t *testing.T, db *sql.DB, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := backfillCommand(t.Context(), store.NewSeasonStore(db), store.NewBackfillStore(db), args, &out)
	return out.String(), err
}
