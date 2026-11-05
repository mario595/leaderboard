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
	"sync"
	"syscall"
	"time"

	"github.com/mario595/leaderboard/internal/activity"
	"github.com/mario595/leaderboard/internal/auth"
	"github.com/mario595/leaderboard/internal/config"
	"github.com/mario595/leaderboard/internal/httpapi"
	"github.com/mario595/leaderboard/internal/logging"
	"github.com/mario595/leaderboard/internal/spa"
	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
	"github.com/mario595/leaderboard/internal/webhook"
	"github.com/mario595/leaderboard/internal/worker"

	"golang.org/x/oauth2"
	"golang.org/x/time/rate"
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
	case "invite":
		if err := runInvite(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "invite failed: %v\n", err)
			os.Exit(1)
		}

	case "backfill":
		if err := runBackfill(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "backfill failed: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintln(os.Stderr, "Invalid command. Use invite, season, backfill or serve")
		os.Exit(2)
	}
}

func runBackfill(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	_, db, err := openDatabase(ctx)
	if err != nil {
		return fmt.Errorf("openDatabase: %w", err)
	}
	defer func() { _ = db.Close() }()

	seasons := store.NewSeasonStore(db)
	jobs := store.NewBackfillStore(db)
	return backfillCommand(ctx, seasons, jobs, args, os.Stdout)
}

func runInvite(args []string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, db, err := openDatabase(ctx)
	if err != nil {
		return fmt.Errorf("openDatabase: %w", err)
	}
	defer func() { _ = db.Close() }()

	st := store.NewInvitationStore(db)
	return inviteCommand(ctx, st, cfg.BaseURL, args, os.Stdout)
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

	if cfg.StravaVerifyToken == "" {
		return fmt.Errorf("STRAVA_VERIFY_TOKEN is required to run the server")
	}

	logger, err := logging.New(os.Stdout, cfg.Env, cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	logger.Info("database ready", "path", cfg.DBPath)

	if cfg.StravaSubscriptionID == 0 {
		logger.Warn("STRAVA_SUBSCRIPTION_ID is not provided. Events will be rejected until it's set.")
	}

	oauthCfg := auth.NewOauthConfig(cfg.StravaClientID, cfg.StravaClientSecret, cfg.BaseURL)
	userStore := store.NewUserStore(db)
	eventStore := store.NewEventStore(db)
	activityService := activity.NewService(db)
	httpClient := &http.Client{Timeout: 10 * time.Second}
	secureCookies := cfg.Env == "prod"
	sessions, stopSessions := auth.NewSessionManager(db, secureCookies)
	defer stopSessions()
	authHandler := auth.New(oauthCfg, userStore, nil, sessions, logger, httpClient, secureCookies)
	wake := make(chan struct{}, 1)

	wh := webhook.New(eventStore, cfg.StravaVerifyToken, cfg.StravaSubscriptionID, wake, logger)
	// One request every 6s, bursts of 5: at most 150 per 15 minutes, under
	// Strava's 200, leaving room for logins. Shared by every Strava client.
	limiter := rate.NewLimiter(rate.Every(6*time.Second), 5)
	api := strava.New(oauthCfg, httpClient, userStore, strava.DefaultBaseURL, limiter)
	newClient := func(userID int64, tok *oauth2.Token) worker.ActivityFetcher {
		return api.ForUser(userID, tok)
	}
	eventWorker := worker.New(eventStore, userStore, activityService, newClient, wake, logger)

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Go(func() {
		if err := eventWorker.Run(workerCtx); err != nil {
			logger.Error("worker stopped", "error", err)
		}
	})
	logger.Info("worker started")
	defer func() {
		cancelWorker()
		wg.Wait()
		logger.Info("worker stopped")
	}()

	activityStore := store.NewActivityStore(db)
	router := httpapi.NewRouter(
		httpapi.Deps{
			Logger:     logger,
			Auth:       authHandler,
			Sessions:   sessions,
			Users:      userStore,
			Webhook:    wh,
			Seasons:    store.NewSeasonStore(db),
			Standings:  activityStore,
			Activities: activityStore,
			SPA:        spa.Handler(),
		},
	)

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
