package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/mario595/leaderboard/internal/store"
)

// seasonResponse is the season part of the leaderboard and activities
// responses; dates are YYYY-MM-DD, both inclusive.
type seasonResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// newSeasonResponse maps a season to its response form.
func newSeasonResponse(s store.Season) seasonResponse {
	return seasonResponse{
		ID:        s.ID,
		Name:      s.Name,
		StartDate: store.FormatDate(s.StartDate),
		EndDate:   store.FormatDate(s.EndDate),
	}
}

// seasonFinder looks up seasons; *store.SeasonStore satisfies it.
type seasonFinder interface {
	Current(ctx context.Context, date time.Time) (store.Season, error)
	Get(ctx context.Context, id int64) (store.Season, error)
}

// selectedSeason returns the season the request asks for: the one covering
// now's UTC date when seasonParam is empty, otherwise the season with that
// ID. A seasonParam that is not a positive integer gives
// errInvalidSeasonParam; a missing season gives an error wrapping
// store.ErrNotFound.
func selectedSeason(ctx context.Context, seasons seasonFinder, now func() time.Time, seasonParam string) (store.Season, error) {
	if seasonParam == "" {
		s, err := seasons.Current(ctx, now().UTC())
		if err != nil {
			return store.Season{}, fmt.Errorf("selected season (current): %w", err)
		}
		return s, nil
	}
	seasonID, err := strconv.ParseInt(seasonParam, 10, 64)
	if err != nil {
		return store.Season{}, errInvalidSeasonParam
	}
	if seasonID <= 0 {
		return store.Season{}, errInvalidSeasonParam
	}
	s, err := seasons.Get(ctx, seasonID)
	if err != nil {
		return store.Season{}, fmt.Errorf("selected season (%d): %w", seasonID, err)
	}

	return s, nil
}

// writeSeasonError answers a failed selectedSeason: 404 when the season does
// not exist (or none covers today), 400 for an invalid ?season=, 500
// otherwise. Shared by every handler that takes ?season=.
func writeSeasonError(w http.ResponseWriter, logger *slog.Logger, err error, seasonParam string) {
	if errors.Is(err, store.ErrNotFound) {
		logger.Info("season not found", "error", err, "season_param", seasonParam)
		writeError(w, logger, http.StatusNotFound, "season not found")
		return
	}
	if errors.Is(err, errInvalidSeasonParam) {
		logger.Info("invalid season", "error", err, "season_param", seasonParam)
		writeError(w, logger, http.StatusBadRequest, "invalid season")
		return
	}
	logger.Error("fetching season", "error", err, "season_param", seasonParam)
	writeError(w, logger, http.StatusInternalServerError, "internal error")
}
