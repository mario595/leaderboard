package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mario595/leaderboard/internal/config"
	"github.com/mario595/leaderboard/internal/httpapi"
	"github.com/mario595/leaderboard/internal/logging"
	"github.com/mario595/leaderboard/internal/store"
)

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "season":
		if err := runSeason(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "season failed: %v\n", err)
			os.Exit(1)
		}
	case "serve":
		if err := runServer(); err != nil {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "Invalid command. Use season or serve")
		os.Exit(2)
	}
}

func runSeason(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_, db, err := openDatabase(ctx)
	if err != nil {
		return fmt.Errorf("openDatabase: %w", err)
	}
	defer func() { _ = db.Close() }()

	st := store.NewSeasonStore(db)
	return seasonCommand(ctx, st, args, os.Stdout)
}

func runServer() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, db, err := openDatabase(ctx)
	if err != nil {
		return fmt.Errorf("openDatabase: %w", err)
	}
	defer func() { _ = db.Close() }()

	if cfg.StravaClientID == "" {
		return fmt.Errorf("STRAVA_CLIENT_ID is required to run the server")
	}

	if cfg.StravaClientSecret == "" {
		return fmt.Errorf("STRAVA_CLIENT_SECRET is required to run the server")
	}

	logger, err := logging.New(os.Stdout, cfg.Env, cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	logger.Info("database ready", "path", cfg.DBPath)

	router := httpapi.NewRouter(logger)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	logger.Info("server starting", "addr", cfg.Addr, "env", cfg.Env)

	select {
	case err := <-errCh:
		return fmt.Errorf("listen: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}

	logger.Info("server stopped")

	return nil
}

// OpenDatabase opens a DB connection. Caller must close it.
func openDatabase(ctx context.Context) (config.Config, *sql.DB, error) {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("load config: %w", err)
	}
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("open db: %w", err)
	}

	err = store.Migrate(ctx, db)
	if err != nil {
		_ = db.Close()
		return config.Config{}, nil, fmt.Errorf("migrate database: %w", err)
	}

	return cfg, db, nil
}
