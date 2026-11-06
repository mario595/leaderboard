package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/mario595/leaderboard/internal/store"
)

func backfillCommand(ctx context.Context, seasons *store.SeasonStore, jobs *store.BackfillStore, args []string, stdout io.Writer) error {
	const usage = "usage: server backfill -season ID"
	fs := flag.NewFlagSet("backfill", flag.ContinueOnError)
	fs.SetOutput(stdout)
	seasonID := fs.Int64("season", 0, "season ID (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *seasonID < 1 {
		return errors.New("-season is required. " + usage)
	}

	season, err := seasons.Get(ctx, *seasonID)
	if err != nil {
		return fmt.Errorf("retrieve season %d: %w", *seasonID, err)
	}

	count, err := jobs.EnqueueAll(ctx, season.ID)
	if err != nil {
		return fmt.Errorf("enqueue for season %d: %w", *seasonID, err)
	}

	_, _ = fmt.Fprintf(stdout, "queued %d backfill jobs for season #%d %q\n", count, season.ID, season.Name)
	return nil
}
