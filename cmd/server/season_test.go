package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mario595/leaderboard/internal/store"
)

func TestSeasonCommand_CreateAndList(t *testing.T) {
	st := newTestSeasonStore(t)

	str, err := runSeasonCmd(t, st, "list")
	if err != nil {
		t.Fatalf("create command: %v", err)
	}
	strings.Contains(str, "no seasons")

	str, err = runSeasonCmd(t, st, "create", "-name", "Autumn 2026", "-start", "2026-09-01", "-end", "2026-12-31")

	if err != nil {
		t.Fatalf("create command: %v", err)
	}
	if !strings.Contains(str, "Autumn 2026") {
		t.Errorf("missing Autumn 2026")
	}
	if !strings.Contains(str, "2026-09-01") {
		t.Errorf("missing 2026-09-01")
	}
	if !strings.Contains(str, "2026-12-31") {
		t.Errorf("missing 2026-12-31")
	}

}

func TestSeasonCommand_listEmpty(t *testing.T) {
	st := newTestSeasonStore(t)

	out, err := runSeasonCmd(t, st, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if want := "no seasons\n"; out != want {
		t.Errorf("list output = %q, want %q", out, want)
	}
}

func TestSeasonCommand_listShowsCreated(t *testing.T) {
	st := newTestSeasonStore(t)

	if _, err := runSeasonCmd(t, st, "create", "-name", "Autumn 2026", "-start", "2026-09-01", "-end", "2026-12-31"); err != nil {
		t.Fatalf("create: %v", err)
	}

	out, err := runSeasonCmd(t, st, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if want := "#1 \"Autumn 2026\" (2026-09-01 to 2026-12-31)\n"; out != want {
		t.Errorf("list output = %q, want %q", out, want)
	}
}

func TestSeasonCommand_errors(t *testing.T) {
	testCases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "no subcommand",
			args:    nil,
			wantErr: "usage",
		},
		{
			name:    "unknown subcommand",
			args:    []string{"nope"},
			wantErr: "nope",
		},
		{
			name:    "unknown flag",
			args:    []string{"create", "-bogus"},
			wantErr: "-bogus",
		},
		{
			name:    "invalid start date",
			args:    []string{"create", "-name", "X", "-start", "2026-13-01", "-end", "2026-12-31"},
			wantErr: "-start",
		},
		{
			name:    "invalid end date",
			args:    []string{"create", "-name", "X", "-start", "2026-09-01", "-end", "nope"},
			wantErr: "-end",
		},
		{
			name:    "missing name",
			args:    []string{"create", "-start", "2026-09-01", "-end", "2026-12-31"},
			wantErr: "Name",
		},
		{
			name:    "end before start",
			args:    []string{"create", "-name", "X", "-start", "2026-12-31", "-end", "2026-09-01"},
			wantErr: "EndDate",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			st := newTestSeasonStore(t)

			_, err := runSeasonCmd(t, st, tc.args...)
			if err == nil {
				t.Fatalf("seasonCommand(%q) succeeded, want an error containing %q", tc.args, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestSeasonCommand_overlap(t *testing.T) {
	st := newTestSeasonStore(t)

	if _, err := runSeasonCmd(t, st, "create", "-name", "Autumn 2026", "-start", "2026-09-01", "-end", "2026-12-31"); err != nil {
		t.Fatalf("create first season: %v", err)
	}

	_, err := runSeasonCmd(t, st, "create", "-name", "Overlap", "-start", "2026-12-01", "-end", "2027-01-31")

	var oe *store.OverlapError
	if !errors.As(err, &oe) {
		t.Fatalf("error = %v, want *store.OverlapError", err)
	}
	if oe.With.Name != "Autumn 2026" {
		t.Errorf("overlapping season = %q, want %q", oe.With.Name, "Autumn 2026")
	}
	if !strings.Contains(err.Error(), "Autumn 2026") {
		t.Errorf("error = %q, want it to mention the existing season", err)
	}

	// The rejected season must not have been stored.
	out, err := runSeasonCmd(t, st, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if strings.Contains(out, "Overlap") {
		t.Errorf("list output = %q, must not contain the rejected season", out)
	}
}

func newTestSeasonStore(t *testing.T) *store.SeasonStore {
	t.Helper()
	db, err := store.Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := store.Migrate(t.Context(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return store.NewSeasonStore(db)
}

func runSeasonCmd(t *testing.T, st *store.SeasonStore, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := seasonCommand(t.Context(), st, args, &out)
	return out.String(), err
}
