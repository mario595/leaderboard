package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(t.Context(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	var fk int
	err = db.QueryRowContext(t.Context(), "PRAGMA foreign_keys").Scan(&fk)

	if err != nil {
		t.Fatalf("queryRowContext PRAGMA foreign_keys error: %v", err)
	}

	if fk != 1 {
		t.Errorf("foreign keys unexpected value. Got %d, wanted 1", fk)
	}

	var mode string
	if err := db.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("queryRowContext PRAGMA journal_mode error: %v", err)
	}

	if mode != "wal" {
		t.Errorf("journal_mode unexpected value. Got %v, wanted wal", mode)
	}
}

func TestOpen_invalidPath(t *testing.T) {
	file := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	bad := filepath.Join(file, "sub", "test.db")

	db, err := Open(t.Context(), bad)
	if err == nil {
		_ = db.Close()
		t.Fatalf("Open(%q) succeeded, want an error", bad)
	}
}
