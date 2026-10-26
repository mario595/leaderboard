package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/mario595/leaderboard/internal/store"
)

func seasonCommand(ctx context.Context, st *store.SeasonStore, args []string, stdout io.Writer) error {
	const usage = "usage: server season create|list"
	if len(args) == 0 {
		return errors.New(usage)
	}

	switch args[0] {
	case "create":
		return createCommand(ctx, st, args[1:], stdout)
	case "list":
		return listCommand(ctx, st, args[1:], stdout)
	default:
		return fmt.Errorf("usage: server season create|list. Unknown command: %v", args[0])
	}

}

func createCommand(ctx context.Context, st *store.SeasonStore, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("season create", flag.ContinueOnError)
	fs.SetOutput(stdout)
	name := fs.String("name", "", "season name (required)")
	start := fs.String("start", "", "start date, YYYY-MM-DD (required)")
	end := fs.String("end", "", "end date, YYYY-MM-DD (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	parsedStart, err := store.ParseDate(*start)
	if err != nil {
		return fmt.Errorf("invalid -start: %w", err)
	}
	parsedEnd, err := store.ParseDate(*end)
	if err != nil {
		return fmt.Errorf("invalid -end: %w", err)
	}

	s, err := st.Create(
		ctx,
		store.Season{
			Name:      *name,
			StartDate: parsedStart,
			EndDate:   parsedEnd,
		},
	)
	if err != nil {
		return fmt.Errorf("create season: %w", err)
	}

	_, _ = fmt.Fprintln(stdout, "created", s)

	return nil
}

func listCommand(ctx context.Context, st *store.SeasonStore, _ []string, stdout io.Writer) error {
	seasons, err := st.List(ctx)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	if len(seasons) == 0 {
		_, _ = fmt.Fprintln(stdout, "no seasons")
	}

	for _, s := range seasons {
		_, _ = fmt.Fprintln(stdout, s)
	}

	return nil
}
