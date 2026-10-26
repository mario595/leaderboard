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

// standingResponse is one row of the leaderboard response.
type standingResponse struct {
	Rank      int    `json:"rank"`
	UserID    int64  `json:"userId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Points    int    `json:"points"`
}

// seasonResponse is the season part of the leaderboard response; dates are
// YYYY-MM-DD, both inclusive.
type seasonResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	StartDate string `json:"startDate"`
	EndDate   string `json:"endDate"`
}

// leaderboardResponse is the body of GET /api/leaderboard.
type leaderboardResponse struct {
	Season    seasonResponse     `json:"season"`
	Standings []standingResponse `json:"standings"`
}

// seasonFinder looks up seasons; *store.SeasonStore satisfies it.
type seasonFinder interface {
	Current(ctx context.Context, date time.Time) (store.Season, error)
	Get(ctx context.Context, id int64) (store.Season, error)
}

// standingsQuerier sums points per member over a date range;
// *store.ActivityStore satisfies it.
type standingsQuerier interface {
	Standings(ctx context.Context, from, to time.Time) ([]store.Standing, error)
}

// errInvalidSeasonParam means ?season= is not a positive integer.
var errInvalidSeasonParam = errors.New("invalid season param")

// leaderboardHandler serves GET /api/leaderboard: the ranked standings of the
// season covering today (UTC date, from now), or of the season given as
// ?season=<id>. It answers 400 when ?season= is not a positive integer, 404
// when there is no current season or the given one does not exist, and 500
// on store errors. It must sit behind requireAuth.
func leaderboardHandler(seasons seasonFinder, standings standingsQuerier, now func() time.Time, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seasonParam := r.URL.Query().Get("season")
		season, err := selectedSeason(r.Context(), seasons, now, seasonParam)
		if err != nil {
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
			return
		}
		stndngs, err := standings.Standings(r.Context(), season.StartDate, season.EndDate)
		if err != nil {
			logger.Error("get standings", "season_id", season.ID, "error", err)
			writeError(w, logger, http.StatusInternalServerError, "internal error")
			return
		}

		lr := leaderboardResponse{
			Season: seasonResponse{
				ID:        season.ID,
				Name:      season.Name,
				StartDate: store.FormatDate(season.StartDate),
				EndDate:   store.FormatDate(season.EndDate),
			},
			Standings: rank(stndngs),
		}
		writeJSON(w, logger, http.StatusOK, lr)
	}
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

// rank turns standings into leaderboard rows with standard competition
// ranking: tied members share a rank and the next rank skips (1, 2, 2, 4).
// standings must be ordered by points, highest first, as
// store.ActivityStore.Standings returns them; ties keep their input order.
// The result is never nil, so it encodes as [] when empty.
func rank(standings []store.Standing) []standingResponse {
	result := make([]standingResponse, 0, len(standings))
	r := 0
	for i, s := range standings {
		if i == 0 || s.Points != standings[i-1].Points {
			r = i + 1
		}
		result = append(result, standingResponse{
			Rank:      r,
			UserID:    s.UserID,
			FirstName: s.FirstName,
			LastName:  s.LastName,
			Points:    s.Points,
		})
	}

	return result
}
