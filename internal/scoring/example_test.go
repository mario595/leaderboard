package scoring_test

import (
	"fmt"
	"time"

	"github.com/mario595/leaderboard/internal/scoring"
)

func ExampleScore() {
	day := func(hour int) time.Time {
		return time.Date(2026, 10, 15, hour, 0, 0, 0, time.UTC)
	}
	activities := []scoring.Activity{
		{ID: 1, SportType: "Run", StartLocal: day(7), MovingTime: 45 * time.Minute},
		{ID: 2, SportType: "Swim", StartLocal: day(12), MovingTime: 40 * time.Minute},
		{ID: 3, SportType: "Ride", StartLocal: day(18), MovingTime: 90 * time.Minute, Description: "Club TT #competition"},
		{ID: 4, SportType: "Walk", StartLocal: day(20), MovingTime: 60 * time.Minute},
	}

	// The ride is a competition (4 points). The run and the swim are worth 2
	// each; only two activities score per day, and the run started earlier.
	// The walk is not a counted sport.
	for _, a := range scoring.Score(activities) {
		fmt.Printf("activity %d: %d points\n", a.ActivityID, a.Points)
	}
	// Output:
	// activity 1: 2 points
	// activity 2: 0 points
	// activity 3: 4 points
	// activity 4: 0 points
}
