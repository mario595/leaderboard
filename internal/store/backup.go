package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Backup writes a copy of the database to a new file at path, while the
// database stays in use. VACUUM INTO reads in one transaction, so the copy is
// consistent and includes what is still in the WAL file, unlike a copy of the
// .db file. SQLite refuses a path that holds a non-empty file. It takes a
// *sql.DB, not a DBTX: VACUUM cannot run inside a transaction.
func Backup(ctx context.Context, db *sql.DB, path string) error {
	_, err := db.ExecContext(ctx, "VACUUM INTO ?", path)
	if err != nil {
		return fmt.Errorf("back up the database to %s: %w", path, err)
	}
	return nil
}
