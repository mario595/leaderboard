package scoring_test

import (
	"testing"

	"github.com/mario595/leaderboard/internal/scoring"
	"github.com/mario595/leaderboard/internal/testutil"
)

func TestDisciplineString(t *testing.T) {
	testCases := []struct {
		discipline scoring.Discipline
		str        string
	}{
		{
			discipline: scoring.DisciplineNone,
			str:        "none",
		},
		{
			discipline: scoring.DisciplineRun,
			str:        "run",
		},
		{
			discipline: scoring.DisciplineRide,
			str:        "ride",
		},
		{
			discipline: scoring.DisciplineSwim,
			str:        "swim",
		},
		{
			discipline: 42,
			str:        "Discipline(42)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.str, func(t *testing.T) {
			testutil.AssertEqual(t, "string", tc.str, tc.discipline.String())
		})
	}
}

func TestDisciplineOf(t *testing.T) {
	testCases := []struct {
		sportType  string
		discipline scoring.Discipline
	}{
		{
			sportType:  "Run",
			discipline: scoring.DisciplineRun,
		},
		{
			sportType:  "TrailRun",
			discipline: scoring.DisciplineRun,
		},
		{
			sportType:  "VirtualRun",
			discipline: scoring.DisciplineRun,
		},
		{
			sportType:  "Ride",
			discipline: scoring.DisciplineRide,
		},
		{
			sportType:  "MountainBikeRide",
			discipline: scoring.DisciplineRide,
		},
		{
			sportType:  "GravelRide",
			discipline: scoring.DisciplineRide,
		},
		{
			sportType:  "VirtualRide",
			discipline: scoring.DisciplineRide,
		},
		{
			sportType:  "Swim",
			discipline: scoring.DisciplineSwim,
		},
		{
			sportType:  "EMountainBikeRide",
			discipline: scoring.DisciplineNone,
		},
		{
			sportType:  "EBikeRide",
			discipline: scoring.DisciplineNone,
		},
		{
			sportType:  "Hike",
			discipline: scoring.DisciplineNone,
		},
		{
			sportType:  "",
			discipline: scoring.DisciplineNone,
		},
		{
			sportType:  "run",
			discipline: scoring.DisciplineNone,
		},
		{
			sportType:  "Walk",
			discipline: scoring.DisciplineNone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.sportType, func(t *testing.T) {
			testutil.AssertEqual(t, "discipline", tc.discipline, scoring.DisciplineOf(tc.sportType))
		})
	}
}
