package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
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

// leaderboardResponse is the body of GET /api/leaderboard.
type leaderboardResponse struct {
	Season    seasonResponse     `json:"season"`
	Standings []standingResponse `json:"standings"`
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
// on store errors (499, logged at Debug, when the client went away). It must
// sit behind requireAuth.
func leaderboardHandler(seasons seasonFinder, standings standingsQuerier, now func() time.Time, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		seasonParam := r.URL.Query().Get("season")
		season, err := selectedSeason(r.Context(), seasons, now, seasonParam)
		if err != nil {
			writeSeasonError(w, r, logger, err, seasonParam)
			return
		}
		stndngs, err := standings.Standings(r.Context(), season.StartDate, season.EndDate)
		if err != nil {
			writeServerError(w, r, logger, "get standings", err, "season_id", season.ID)
			return
		}

		lr := leaderboardResponse{
			Season:    newSeasonResponse(season),
			Standings: rank(stndngs),
		}
		writeJSON(w, logger, http.StatusOK, lr)
	}
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
