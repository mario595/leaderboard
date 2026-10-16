package store

import (
	"database/sql"
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

	assertTableExists(t, db, "users")
	assertTableExists(t, db, "seasons")
	assertTableExists(t, db, "sessions")
	assertTableExists(t, db, "activities")

	// Run a second time. Should not fail (idempotency).
	if err := Migrate(t.Context(), db); err != nil {
		t.Fatalf("Migrate(): %v", err)
	}
}

func assertTableExists(t *testing.T, db *sql.DB, tableName string) {
	t.Helper()
	var name string
	err := db.QueryRowContext(t.Context(), "SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&name)
	if err != nil {
		t.Fatalf("QueryRowContext(): %v", err)
	}

	if name != tableName {
		t.Errorf("Unexpected table name. Wanted %v. Got %v", tableName, name)
	}
}
