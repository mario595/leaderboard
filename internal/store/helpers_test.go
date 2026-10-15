package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func assertEqual[T comparable](t *testing.T, name string, want, got T) {
	t.Helper()
	if want != got {
		t.Errorf("unexpected value %s: wanted %v, got %v", name, want, got)
	}
}

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

func assertTime(t *testing.T, propertyName string, wanted, got time.Time) {
	t.Helper()
	if !wanted.Truncate(time.Second).Equal(got) {
		t.Errorf("unexpected value time %s: wanted %v, got %v", propertyName, wanted, got)
	}
}
