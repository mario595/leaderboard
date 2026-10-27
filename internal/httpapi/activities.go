package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mario595/leaderboard/internal/scoring"
	"github.com/mario595/leaderboard/internal/store"
)

// cursorTimeLayout is the time format inside a cursor. It is part of the API's
// cursor format, separate from how the store saves start times, even though
// both look the same today.
const cursorTimeLayout = "2006-01-02T15:04:05"

// errInvalidCursor means a cursor could not be decoded. Clients cannot fix an
// opaque cursor, so it carries no detail: they start again from page one.
var errInvalidCursor = errors.New("invalid cursor")

// activityLister pages through a user's activities; *store.ActivityStore
// satisfies it.
type activityLister interface {
	ListForUser(ctx context.Context, userID int64, from, to time.Time, after *store.ActivityCursor, limit int) ([]store.Activity, error)
}

// activityResponse is one activity of GET /api/me/activities. MovingTime is
// in seconds and Distance in metres; Competition follows the scoring rules,
// so it is false for sports that do not count. The description is never
// returned.
type activityResponse struct {
	ID          int64   `json:"id"`
	Date        string  `json:"date"`
	StartLocal  string  `json:"startLocal"`
	SportType   string  `json:"sportType"`
	MovingTime  int64   `json:"movingTime"`
	Distance    float64 `json:"distance"`
	Competition bool    `json:"competition"`
	Points      int     `json:"points"`
}

// activitiesResponse is the body of GET /api/me/activities. NextCursor is
// null on the last page.
type activitiesResponse struct {
	Season     seasonResponse     `json:"season"`
	Activities []activityResponse `json:"activities"`
	NextCursor *string            `json:"nextCursor"`
}

// errInvalidLimit means ?limit= is not an integer between 1 and 100.
var errInvalidLimit = errors.New("invalid limit")

// activitiesHandler serves GET /api/me/activities: one page of the logged-in
// user's activities in the selected season (as for the leaderboard), newest
// first. ?limit= sets the page size (1 to 100, default 50) and ?cursor=
// continues after a previous page. It reads one row more than the limit to
// know whether a next page exists. 400 for a bad limit, cursor or season,
// 404 for a missing season, 500 on store errors. It must sit behind
// requireAuth: the user always comes from the session, never from a
// parameter.
func activitiesHandler(seasons seasonFinder, activities activityLister, now func() time.Time, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, ok := userFromContext(r.Context())
		if !ok {
			logger.Error("missing user in activitiesHandler")
			writeError(w, logger, http.StatusInternalServerError, "internal error")
			return
		}
		limit, err := parseLimit(r.URL.Query().Get("limit"))
		if err != nil {
			writeError(w, logger, http.StatusBadRequest, "invalid limit")
			return
		}

		cursorParam := r.URL.Query().Get("cursor")
		var after *store.ActivityCursor
		if cursorParam != "" {
			crsr, err := decodeCursor(cursorParam)
			if err != nil {
				writeError(w, logger, http.StatusBadRequest, "invalid cursor")
				return
			}
			after = &crsr
		}

		seasonParam := r.URL.Query().Get("season")
		season, err := selectedSeason(r.Context(), seasons, now, seasonParam)
		if err != nil {
			writeSeasonError(w, logger, err, seasonParam)
			return
		}

		a, err := activities.ListForUser(
			r.Context(),
			u.ID,
			season.StartDate,
			season.EndDate,
			after,
			limit+1,
		)

		if err != nil {
			logger.Error("list activities", "user_id", u.ID, "error", err)
			writeError(w, logger, http.StatusInternalServerError, "internal error")
			return
		}

		var nextCursor *string
		if len(a) > limit {
			lastActivity := a[limit-1]
			enc := encodeCursor(store.ActivityCursor{
				StartLocal: lastActivity.StartLocal,
				StravaID:   lastActivity.StravaID,
			})
			nextCursor = &enc
			a = a[:limit]
		}

		var activitiesRes = make([]activityResponse, 0, len(a))

		for _, act := range a {
			activitiesRes = append(activitiesRes, activityResponse{
				ID:         act.StravaID,
				Date:       act.Day(),
				StartLocal: act.StartLocal.Format(cursorTimeLayout),
				SportType:  act.SportType,
				Distance:   act.Distance,
				MovingTime: int64(act.MovingTime / time.Second),
				Competition: scoring.Activity{
					SportType:   act.SportType,
					WorkoutType: act.WorkoutType,
					Description: act.Description,
				}.IsCompetition(),
				Points: act.Points,
			})
		}

		writeJSON(w, logger, http.StatusOK, activitiesResponse{
			Activities: activitiesRes,
			NextCursor: nextCursor,
			Season:     newSeasonResponse(season),
		})
	}
}

// parseLimit returns the page size: 50 when limitParam is empty, otherwise
// its value if it is an integer from 1 to 100, else errInvalidLimit.
func parseLimit(limitParam string) (int, error) {
	l := 50
	var err error
	if limitParam != "" {
		l, err = strconv.Atoi(limitParam)
		if err != nil || l <= 0 || l > 100 {
			return 0, errInvalidLimit
		}
	}
	return l, nil
}

// encodeCursor turns a position in the activity list into an opaque,
// URL-safe string (base64 without padding of "start|id"). Clients must pass it
// back unchanged and never build or read one.
func encodeCursor(c store.ActivityCursor) string {
	str := fmt.Sprintf("%s|%d", c.StartLocal.Format(cursorTimeLayout), c.StravaID)
	return base64.RawURLEncoding.EncodeToString([]byte(str))
}

// decodeCursor reverses encodeCursor. Any malformed input, including a
// non-positive ID, gives errInvalidCursor.
func decodeCursor(s string) (store.ActivityCursor, error) {
	dec, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return store.ActivityCursor{}, errInvalidCursor
	}

	b, a, found := strings.Cut(string(dec), "|")
	if !found {
		return store.ActivityCursor{}, errInvalidCursor
	}

	startLocal, err := time.Parse(cursorTimeLayout, b)
	if err != nil {
		return store.ActivityCursor{}, errInvalidCursor
	}
	stravaID, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return store.ActivityCursor{}, errInvalidCursor
	}
	if stravaID <= 0 {
		return store.ActivityCursor{}, errInvalidCursor
	}
	return store.ActivityCursor{
		StartLocal: startLocal,
		StravaID:   stravaID,
	}, nil
}
