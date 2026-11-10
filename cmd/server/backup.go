package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/mario595/leaderboard/internal/store"
)

// backupCommand writes a consistent copy of the database to the -out path,
// which must not exist: an existing file may be an earlier backup.
func backupCommand(ctx context.Context, db *sql.DB, args []string, stdout io.Writer) error {
	const usage = "usage: server backup -out PATH"
	flags := flag.NewFlagSet("backup", flag.ContinueOnError)
	flags.SetOutput(stdout)
	out := flags.String("out", "", "path of the new backup file (required)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("-out is required. " + usage)
	}

	// SQLite refuses a non-empty file but writes into an empty one; this check
	// covers both, with a clear message. Another process could create the file
	// between the check and the backup: SQLite still refuses it then.
	_, err := os.Stat(*out)
	if err == nil {
		return fmt.Errorf("%s already exists", *out)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("check %s: %w", *out, err)
	}

	if err := store.Backup(ctx, db, *out); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "backup written to %s\n", *out)
	return nil
}
