package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrNotFound is returned when a requested row does not exist. Check it with errors.Is.
var ErrNotFound = errors.New("not found")

// rowScanner is implemented by *sql.Row and *sql.Rows, so one scan function serves both.
type rowScanner interface {
	Scan(dest ...any) error
}

// DBTX is the query subset shared by *sql.DB and *sql.Tx, so a store can run
// inside or outside a transaction.
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
