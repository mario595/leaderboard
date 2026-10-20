package scoring_test

import (
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/scoring"
	"github.com/mario595/leaderboard/internal/testutil"
)

// baseActivity returns an eligible, non-competition run worth BasePoints.
// Each test row starts from a fresh copy and changes only the fields it is about.
func baseActivity() scoring.Activity {
	return scoring.Activity{
		ID:          12,
		SportType:   "Run",
		StartLocal:  time.Date(2026, 11, 12, 7, 30, 0, 0, time.UTC),
		MovingTime:  95 * time.Minute,
		WorkoutType: 0,
		Description: "Morning run",
	}
}

func TestActivityEligible(t *testing.T) {
	testCases := []struct {
		name       string
		sportType  string
		movingTime time.Duration
		want       bool
	}{
		// Positive control: the fixture itself must be eligible.
		{name: "base fixture", sportType: "Run", movingTime: 95 * time.Minute, want: true},

		{name: "TrailRun", sportType: "TrailRun", movingTime: 45 * time.Minute, want: true},
		{name: "VirtualRun", sportType: "VirtualRun", movingTime: 45 * time.Minute, want: true},
		{name: "Ride", sportType: "Ride", movingTime: 45 * time.Minute, want: true},
		{name: "MountainBikeRide", sportType: "MountainBikeRide", movingTime: 45 * time.Minute, want: true},
		{name: "GravelRide", sportType: "GravelRide", movingTime: 45 * time.Minute, want: true},
		{name: "VirtualRide", sportType: "VirtualRide", movingTime: 45 * time.Minute, want: true},
		{name: "Swim", sportType: "Swim", movingTime: 45 * time.Minute, want: true},

		{name: "EBikeRide not counted", sportType: "EBikeRide", movingTime: 45 * time.Minute, want: false},
		{name: "Walk not counted", sportType: "Walk", movingTime: 45 * time.Minute, want: false},

		// Boundary around MinMovingTime.
		{name: "29m59s too short", sportType: "Run", movingTime: 29*time.Minute + 59*time.Second, want: false},
		{name: "exactly 30m", sportType: "Run", movingTime: 30 * time.Minute, want: true},
		{name: "30m1s", sportType: "Run", movingTime: 30*time.Minute + time.Second, want: true},
		{name: "zero moving time", sportType: "Run", movingTime: 0, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := baseActivity()
			a.SportType = tc.sportType
			a.MovingTime = tc.movingTime

			testutil.AssertEqual(t, "eligible", tc.want, a.Eligible())
		})
	}
}

func TestActivityIsCompetition(t *testing.T) {
	testCases := []struct {
		name        string
		sportType   string
		workoutType int
		description string
		want        bool
	}{
		// Positive control: the fixture itself is not a competition.
		{name: "base fixture", sportType: "Run", workoutType: 0, description: "Morning run", want: false},

		// Workout type: race codes depend on the discipline. Literal Strava codes,
		// not the package constants, so a wrong constant fails here.
		{name: "run race", sportType: "Run", workoutType: 1, want: true},
		{name: "trail run race", sportType: "TrailRun", workoutType: 1, want: true},
		{name: "run long run", sportType: "Run", workoutType: 2, want: false},
		{name: "run workout", sportType: "Run", workoutType: 3, want: false},
		{name: "run with ride race code", sportType: "Run", workoutType: 11, want: false},
		{name: "ride race", sportType: "Ride", workoutType: 11, want: true},
		{name: "gravel ride race", sportType: "GravelRide", workoutType: 11, want: true},
		{name: "ride default", sportType: "Ride", workoutType: 10, want: false},
		{name: "ride workout", sportType: "Ride", workoutType: 12, want: false},
		{name: "ride with run race code", sportType: "Ride", workoutType: 1, want: false},
		{name: "swim with run race code", sportType: "Swim", workoutType: 1, want: false},

		// Description tag: any discipline, case-insensitive, whole word.
		{name: "swim with tag", sportType: "Swim", description: "Open water #competition", want: true},
		{name: "tag in sentence", sportType: "Run", description: "Race day #competition", want: true},
		{name: "tag only", sportType: "Run", description: "#competition", want: true},
		{name: "tag capitalised", sportType: "Run", description: "#Competition", want: true},
		{name: "tag upper case with punctuation", sportType: "Run", description: "#COMPETITION!", want: true},
		{name: "tag followed by period", sportType: "Run", description: "Done. #competition.", want: true},
		{name: "tag on second line", sportType: "Run", description: "line one\n#competition", want: true},
		{name: "plural is not the tag", sportType: "Run", description: "#competitions", want: false},
		{name: "word without hash", sportType: "Run", description: "competition", want: false},
		{name: "tag glued to a word", sportType: "Run", description: "x#competition", want: false},
		{name: "empty description", sportType: "Run", description: "", want: false},

		// Sports that do not count are never competitions, even with the tag.
		{name: "walk with tag", sportType: "Walk", description: "#competition", want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := baseActivity()
			a.SportType = tc.sportType
			a.WorkoutType = tc.workoutType
			a.Description = tc.description

			testutil.AssertEqual(t, "competition", tc.want, a.IsCompetition())
		})
	}
}

func TestActivityPoints(t *testing.T) {
	testCases := []struct {
		name        string
		sportType   string
		movingTime  time.Duration
		workoutType int
		description string
		want        int
	}{
		// Positive control: the fixture scores base points. Literal point values
		// (the rules), not the package constants, so a wrong constant fails here.
		{name: "base fixture", sportType: "Run", movingTime: 95 * time.Minute, want: 2},

		{name: "race by workout type", sportType: "Run", movingTime: 45 * time.Minute, workoutType: 1, want: 4},
		{name: "race by tag", sportType: "Swim", movingTime: 45 * time.Minute, description: "#competition", want: 4},
		{name: "short race scores nothing", sportType: "Run", movingTime: 20 * time.Minute, workoutType: 1, want: 0},
		{name: "e-bike with tag scores nothing", sportType: "EBikeRide", movingTime: 45 * time.Minute, description: "#competition", want: 0},
		{name: "not counted sport", sportType: "Walk", movingTime: 45 * time.Minute, want: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := baseActivity()
			a.SportType = tc.sportType
			a.MovingTime = tc.movingTime
			a.WorkoutType = tc.workoutType
			a.Description = tc.description

			testutil.AssertEqual(t, "points", tc.want, a.Points())
		})
	}
}

func TestActivityDay(t *testing.T) {
	testCases := []struct {
		name  string
		start time.Time
		want  string
	}{
		{name: "morning", start: time.Date(2026, 10, 15, 7, 30, 0, 0, time.UTC), want: "2026-10-15"},
		{name: "last minute of the day", start: time.Date(2026, 10, 15, 23, 59, 0, 0, time.UTC), want: "2026-10-15"},
		{name: "midnight starts a new day", start: time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), want: "2026-10-16"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := baseActivity()
			a.StartLocal = tc.start

			testutil.AssertEqual(t, "day", tc.want, a.Day())
		})
	}
}
