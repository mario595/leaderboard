package scoring

import (
	"cmp"
	"slices"
)

// Award is the points awarded to one activity after the daily cap.
type Award struct {
	ActivityID int64
	Points     int
}

// Score applies the daily cap to one user's activities and returns one Award
// per activity, in input order. The activities may span any number of days.
//
// For each day (see Activity.Day), the MaxScoringPerDay eligible activities
// with the most points score their Points; ties go to the earlier start time,
// then the lower ID. All other activities, including ineligible ones, get 0.
//
// Score knows nothing about users: the caller passes the activities of a
// single user. The input slice is not modified. A nil or empty input returns
// an empty, non-nil slice.
func Score(activities []Activity) []Award {
	awards := make([]Award, len(activities))

	byDay := make(map[string][]int)
	for i, a := range activities {
		awards[i].ActivityID = a.ID
		if a.Eligible() {
			byDay[a.Day()] = append(byDay[a.Day()], i)
		}
	}

	for _, indices := range byDay {
		slices.SortFunc(indices, func(x, y int) int {
			a, b := activities[x], activities[y]
			return cmp.Or(
				cmp.Compare(b.Points(), a.Points()),
				a.StartLocal.Compare(b.StartLocal),
				cmp.Compare(a.ID, b.ID),
			)
		})

		for _, i := range indices[:min(len(indices), MaxScoringPerDay)] {
			awards[i].Points = activities[i].Points()
		}
	}
	return awards
}
