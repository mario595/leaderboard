package scoring_test

import (
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/scoring"
)

func TestScore(t *testing.T) {
	testCases := []struct {
		name       string
		activities []scoring.Activity
		wanted     []scoring.Award
	}{
		{
			name:       "nil list",
			activities: nil,
			wanted:     []scoring.Award{},
		},
		{
			name:       "empty list",
			activities: []scoring.Activity{},
			wanted:     []scoring.Award{},
		},
		{
			name: "ineligible activities score nothing",
			activities: []scoring.Activity{
				{ID: 1, SportType: "EBikeRide", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)},
				{ID: 2, SportType: "EBikeRide", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)},
			},
			wanted: []scoring.Award{
				{ActivityID: 1, Points: 0},
				{ActivityID: 2, Points: 0},
			},
		},
		{
			name: "one eligible activity per day",
			activities: []scoring.Activity{
				{ID: 1, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)},
				{ID: 2, SportType: "Ride", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)},
			},
			wanted: []scoring.Award{
				{ActivityID: 1, Points: 2},
				{ActivityID: 2, Points: 2},
			},
		},
		{
			name: "two eligible activities per day",
			activities: []scoring.Activity{
				{ID: 1, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)},
				{ID: 2, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)},

				{ID: 3, SportType: "Ride", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)},
				{ID: 4, SportType: "Ride", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 16, 0, 0, 0, time.UTC)},
			},
			wanted: []scoring.Award{
				{ActivityID: 1, Points: 2},
				{ActivityID: 2, Points: 2},
				{ActivityID: 3, Points: 2},
				{ActivityID: 4, Points: 2},
			},
		},
		{
			name: "ineligible activities do not take a daily slot",
			activities: []scoring.Activity{
				{ID: 1, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)},
				{ID: 2, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)},
				{ID: 3, SportType: "Run", MovingTime: 10 * time.Minute, StartLocal: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)},

				{ID: 4, SportType: "Ride", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)},
				{ID: 5, SportType: "Ride", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 16, 0, 0, 0, time.UTC)},
				{ID: 6, SportType: "Ride", MovingTime: 10 * time.Minute, StartLocal: time.Date(2026, 11, 13, 16, 0, 0, 0, time.UTC)},
			},
			wanted: []scoring.Award{
				{ActivityID: 1, Points: 2},
				{ActivityID: 2, Points: 2},
				{ActivityID: 3, Points: 0},
				{ActivityID: 4, Points: 2},
				{ActivityID: 5, Points: 2},
				{ActivityID: 6, Points: 0},
			},
		},
		{
			name: "cap keeps the highest points",
			activities: []scoring.Activity{
				{ID: 1, SportType: "Run", WorkoutType: scoring.RunRaceWorkoutType, MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)},
				{ID: 2, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)},
				{ID: 3, SportType: "Run", MovingTime: 60 * time.Minute, StartLocal: time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC)},

				{ID: 4, SportType: "Ride", WorkoutType: scoring.RideRaceWorkoutType, MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)},
				{ID: 5, SportType: "Ride", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 16, 0, 0, 0, time.UTC)},
				{ID: 6, SportType: "Ride", MovingTime: 60 * time.Minute, StartLocal: time.Date(2026, 11, 13, 17, 0, 0, 0, time.UTC)},
			},
			wanted: []scoring.Award{
				{ActivityID: 1, Points: 4},
				{ActivityID: 2, Points: 2},
				{ActivityID: 3, Points: 0},
				{ActivityID: 4, Points: 4},
				{ActivityID: 5, Points: 2},
				{ActivityID: 6, Points: 0},
			},
		},
		{
			name: "cap breaks ties by start time, then ID",
			activities: []scoring.Activity{
				{ID: 1, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC)},
				{ID: 2, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)},
				{ID: 3, SportType: "Run", MovingTime: 60 * time.Minute, StartLocal: time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)},

				{ID: 6, SportType: "Ride", MovingTime: 60 * time.Minute, StartLocal: time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)},
				{ID: 5, SportType: "Ride", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)},
				{ID: 4, SportType: "Ride", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)},
			},
			wanted: []scoring.Award{
				{ActivityID: 1, Points: 0},
				{ActivityID: 2, Points: 2},
				{ActivityID: 3, Points: 2},
				{ActivityID: 6, Points: 0},
				{ActivityID: 5, Points: 2},
				{ActivityID: 4, Points: 2},
			},
		},
		{
			// Days interleaved in the input: results must follow input order,
			// not the order days are grouped or iterated in.
			name: "days interleaved in input keep input order",
			activities: []scoring.Activity{
				{ID: 1, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)},
				{ID: 2, SportType: "Ride", WorkoutType: 11, MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 15, 0, 0, 0, time.UTC)},
				{ID: 3, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)},
				{ID: 4, SportType: "Ride", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 13, 16, 0, 0, 0, time.UTC)},
				{ID: 5, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC)},
			},
			wanted: []scoring.Award{
				{ActivityID: 1, Points: 2},
				{ActivityID: 2, Points: 4},
				{ActivityID: 3, Points: 2},
				{ActivityID: 4, Points: 2},
				{ActivityID: 5, Points: 0},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := scoring.Score(tc.activities)
			if !reflect.DeepEqual(got, tc.wanted) {
				t.Errorf("got %+v, wanted %+v", got, tc.wanted)
			}
		})
	}
}

func TestScoreDoesNotModifyInput(t *testing.T) {
	// Out of scoring order on purpose: sorting the input in place would move
	// the later, higher-points race to the front.
	activities := []scoring.Activity{
		{ID: 1, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 15, 0, 0, 0, time.UTC)},
		{ID: 2, SportType: "Run", MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 16, 0, 0, 0, time.UTC)},
		{ID: 3, SportType: "Run", WorkoutType: 1, MovingTime: 45 * time.Minute, StartLocal: time.Date(2026, 11, 12, 17, 0, 0, 0, time.UTC)},
	}
	before := slices.Clone(activities)

	scoring.Score(activities)

	if !reflect.DeepEqual(before, activities) {
		t.Errorf("input modified: got %+v, wanted %+v", activities, before)
	}
}
