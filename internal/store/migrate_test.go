package store

import (
	"path/filepath"
	"testing"
)

func TestMigrate(t *testing.T) {
	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(t.Context(), db); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}

	var name string
	err = db.QueryRowContext(t.Context(), "SELECT name FROM sqlite_master WHERE type='table' AND name='users'").Scan(&name)
	if err != nil {
		t.Fatalf("QueryRowContext(): %v", err)
	}

	if name != "users" {
		t.Errorf("Unexpected table name. Wanted users. Got %v", name)
	}

	// Run a second time. Should not fail (idempotency).
	if err := Migrate(t.Context(), db); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}
}
