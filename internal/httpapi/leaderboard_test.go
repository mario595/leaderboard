package httpapi

import (
	"slices"
	"testing"

	"github.com/mario595/leaderboard/internal/store"
)

func TestRank(t *testing.T) {
	testCases := []struct {
		name      string
		points    []int // already ordered, highest first, as Standings returns them
		wantRanks []int
	}{
		{name: "no ties", points: []int{6, 4, 2}, wantRanks: []int{1, 2, 3}},
		// Standard competition ranking: tied members share a rank and the
		// next rank skips (1, 1, 3), it does not continue (1, 1, 2).
		{name: "tie at the top", points: []int{6, 6, 4}, wantRanks: []int{1, 1, 3}},
		{name: "tie in the middle", points: []int{6, 4, 4, 2}, wantRanks: []int{1, 2, 2, 4}},
		{name: "three-way tie", points: []int{4, 4, 4, 2}, wantRanks: []int{1, 1, 1, 4}},
		{name: "tie at the bottom", points: []int{6, 0, 0}, wantRanks: []int{1, 2, 2}},
		{name: "everyone at zero", points: []int{0, 0}, wantRanks: []int{1, 1}},
		{name: "one member", points: []int{2}, wantRanks: []int{1}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			standings := make([]store.Standing, len(tc.points))
			for i, p := range tc.points {
				standings[i] = store.Standing{UserID: int64(i + 1), FirstName: "F", LastName: "L", Points: p}
			}

			got := rank(standings)

			gotRanks := make([]int, len(got))
			for i, r := range got {
				gotRanks[i] = r.Rank
			}
			if !slices.Equal(tc.wantRanks, gotRanks) {
				t.Errorf("ranks for points %v: got %v, wanted %v", tc.points, gotRanks, tc.wantRanks)
			}
		})
	}
}

func TestRank_copiesEveryFieldInOrder(t *testing.T) {
	standings := []store.Standing{
		{UserID: 7, FirstName: "Ada", LastName: "Lovelace", Points: 24},
		{UserID: 9, FirstName: "Alan", LastName: "Turing", Points: 18},
	}

	got := rank(standings)

	want := []standingResponse{
		{Rank: 1, UserID: 7, FirstName: "Ada", LastName: "Lovelace", Points: 24},
		{Rank: 2, UserID: 9, FirstName: "Alan", LastName: "Turing", Points: 18},
	}
	if !slices.Equal(want, got) {
		t.Errorf("rank:\n got  %+v\n want %+v", got, want)
	}
}

func TestRank_empty(t *testing.T) {
	for _, in := range [][]store.Standing{nil, {}} {
		got := rank(in)
		// Encoded as [] in the response, not null.
		if got == nil || len(got) != 0 {
			t.Errorf("rank(%#v): got %#v, wanted an empty non-nil slice", in, got)
		}
	}
}
