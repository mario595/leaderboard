package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := Open(t.Context(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open DB: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	if err = Migrate(t.Context(), db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return db
}
