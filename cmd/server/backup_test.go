package main

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

// The backup is a database of its own with the same data, written while the
// source stays open. The season is still in the source's WAL file: a plain
// file copy of leaderboard.db would miss it.
func TestBackupCommand(t *testing.T) {
	db := newTestDB(t)
	createCLITestSeason(t, db)
	out := filepath.Join(t.TempDir(), "backup.db")

	stdout, err := runBackupCmd(t, db, "-out", out)
	if err != nil {
		t.Fatalf("backup: %v", err)
	}

	if !strings.Contains(stdout, out) {
		t.Errorf("output %q does not name the backup %q", stdout, out)
	}
	backup, err := store.Open(t.Context(), out)
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	t.Cleanup(func() { _ = backup.Close() })
	seasons, err := store.NewSeasonStore(backup).List(t.Context())
	if err != nil {
		t.Fatalf("list seasons in the backup: %v", err)
	}
	testutil.AssertEqual(t, "seasons in the backup", 1, len(seasons))
	testutil.AssertEqual(t, "season name", "Autumn 2026", seasons[0].Name)
}

// An existing file is never overwritten: it may be an earlier backup. SQLite
// refuses a non-empty file on its own but writes into an empty one, so the
// empty file is what proves the command's own check.
func TestBackupCommand_existingFile(t *testing.T) {
	for _, content := range []string{"an earlier backup", ""} {
		t.Run("content "+strconv.Quote(content), func(t *testing.T) {
			db := newTestDB(t)
			out := filepath.Join(t.TempDir(), "backup.db")
			if err := os.WriteFile(out, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}

			if _, err := runBackupCmd(t, db, "-out", out); err == nil {
				t.Fatal("backup over an existing file: got nil error")
			}

			got, err := os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			testutil.AssertEqual(t, "existing file", content, string(got))
		})
	}
}

func TestBackupCommand_errors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no -out", args: nil},
		{name: "empty -out", args: []string{"-out", ""}},
		{name: "unknown flag", args: []string{"-to", "x.db"}},
		{name: "folder does not exist", args: []string{"-out", filepath.Join(t.TempDir(), "missing", "backup.db")}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := runBackupCmd(t, newTestDB(t), tc.args...); err == nil {
				t.Errorf("backup %v: got nil error", tc.args)
			}
		})
	}
}

func runBackupCmd(t *testing.T, db *sql.DB, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := backupCommand(t.Context(), db, args, &out)
	return out.String(), err
}
