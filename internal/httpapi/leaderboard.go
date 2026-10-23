package httpapi

import "github.com/mario595/leaderboard/internal/store"

// standingResponse is one row of the leaderboard response.
type standingResponse struct {
	Rank      int    `json:"rank"`
	UserID    int64  `json:"userId"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Points    int    `json:"points"`
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
