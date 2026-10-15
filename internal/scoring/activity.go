package scoring

import (
	"strings"
	"time"
)

// Scoring rules.
const (
	// MinMovingTime is the minimum moving time for an activity to score.
	MinMovingTime = 30 * time.Minute
	// BasePoints is the points for an eligible activity that is not a competition.
	BasePoints = 2
	// CompetitionPoints is the points for an eligible competition.
	CompetitionPoints = 4
	// MaxScoringPerDay is the maximum number of activities that score per user per day.
	MaxScoringPerDay = 2
	// CompetitionTag marks a competition when it appears as a word in the
	// activity description. It is our convention, typed by athletes; Strava
	// does not know about it. Matched case-insensitively.
	CompetitionTag = "#competition"
)

// Strava workout_type codes that mark a race. Swims have no race code.
const (
	// RunRaceWorkoutType is the workout_type of a race for runs.
	RunRaceWorkoutType = 1
	// RideRaceWorkoutType is the workout_type of a race for rides.
	RideRaceWorkoutType = 11
)

// Activity holds the fields of a Strava activity that the scoring rules need.
type Activity struct {
	// ID is the Strava activity ID.
	ID int64
	// SportType is the Strava sport_type, for example "Run" or "GravelRide".
	SportType string
	// StartLocal is the athlete's local wall-clock start time (Strava
	// start_date_local). Its location is irrelevant; only the date and clock
	// fields are used.
	StartLocal time.Time
	// MovingTime is the Strava moving_time.
	MovingTime time.Duration
	// WorkoutType is the Strava workout_type, 0 when Strava returns null.
	WorkoutType int
	// Description is the Strava activity description, searched for CompetitionTag.
	Description string
}

// Day returns the local start date as YYYY-MM-DD. The daily cap groups
// activities by this value.
func (a Activity) Day() string {
	return a.StartLocal.Format(time.DateOnly)
}

// Eligible reports whether a's sport type is counted and its moving time is
// at least MinMovingTime.
func (a Activity) Eligible() bool {
	return DisciplineOf(a.SportType) != DisciplineNone && a.MovingTime >= MinMovingTime
}

// IsCompetition reports whether a is a race for its discipline (Strava
// workout_type) or has CompetitionTag in its description. Activities of a
// sport type that does not count are never competitions. It does not check
// moving time; use Points for the awarded value.
func (a Activity) IsCompetition() bool {
	discipline := DisciplineOf(a.SportType)
	switch discipline {
	case DisciplineRun:
		return a.WorkoutType == RunRaceWorkoutType || hasCompetitionTag(a.Description)
	case DisciplineRide:
		return a.WorkoutType == RideRaceWorkoutType || hasCompetitionTag(a.Description)
	case DisciplineSwim:
		return hasCompetitionTag(a.Description)
	}
	return false
}

// Points returns the points a is worth before the daily cap: 0 if not
// eligible, CompetitionPoints for a competition, BasePoints otherwise.
func (a Activity) Points() int {
	if !a.Eligible() {
		return 0
	}
	if a.IsCompetition() {
		return CompetitionPoints
	}
	return BasePoints
}

// hasCompetitionTag reports whether description contains CompetitionTag as a
// whole word, ignoring case and trailing punctuation.
func hasCompetitionTag(description string) bool {
	words := strings.Fields(description)
	for _, w := range words {
		trim := strings.TrimRight(w, ".,;:!?")
		if strings.EqualFold(trim, CompetitionTag) {
			return true
		}
	}
	return false
}
