package scoring

import "fmt"

// Discipline is the family of sports a Strava sport type belongs to.
type Discipline int

const (
	DisciplineNone Discipline = iota // not counted; the zero value
	DisciplineRun
	DisciplineRide
	DisciplineSwim
)

// String returns the lowercase name of d.
func (d Discipline) String() string {
	switch d {
	case DisciplineNone:
		return "none"
	case DisciplineRun:
		return "run"
	case DisciplineRide:
		return "ride"
	case DisciplineSwim:
		return "swim"
	default:
		return fmt.Sprintf("Discipline(%d)", int(d))
	}
}

// DisciplineOf maps a Strava sport_type to its Discipline, or DisciplineNone if the sport does not score.
func DisciplineOf(sportType string) Discipline {
	switch sportType {
	case "Run", "TrailRun", "VirtualRun":
		return DisciplineRun
	case "Ride", "MountainBikeRide", "GravelRide", "VirtualRide":
		return DisciplineRide
	case "Swim":
		return DisciplineSwim
	default:
		return DisciplineNone
	}
}
