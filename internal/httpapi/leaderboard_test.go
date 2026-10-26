package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
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

func TestLeaderboardHandler_currentSeason(t *testing.T) {
	f := newLeaderboardFixture(t)

	rec := f.get(t, "/api/leaderboard")

	testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
	// "Today" is the injected clock's UTC date, and the standings use the
	// season's own range.
	testutil.AssertEqual(t, "Current asked for", "2026-10-15", store.FormatDate(f.seasons.askedDate))
	testutil.AssertEqual(t, "Get calls", 0, len(f.seasons.askedIDs))
	testutil.AssertEqual(t, "standings from", "2026-09-01", store.FormatDate(f.standings.from))
	testutil.AssertEqual(t, "standings to", "2026-12-31", store.FormatDate(f.standings.to))
	assertJSON(t, rec.Body.String(), `{
		"season": {"id": 3, "name": "Autumn 2026", "startDate": "2026-09-01", "endDate": "2026-12-31"},
		"standings": [
			{"rank": 1, "userId": 7, "firstName": "Ada", "lastName": "Lovelace", "points": 24},
			{"rank": 2, "userId": 9, "firstName": "Alan", "lastName": "Turing", "points": 18},
			{"rank": 2, "userId": 4, "firstName": "Grace", "lastName": "Hopper", "points": 18}
		]
	}`)
}

func TestLeaderboardHandler_todayIsTheUTCDate(t *testing.T) {
	f := newLeaderboardFixture(t)
	// 01:00 on the 16th in UTC+14 is still the 15th in UTC.
	f.now = time.Date(2026, 10, 16, 1, 0, 0, 0, time.FixedZone("UTC+14", 14*3600))

	f.get(t, "/api/leaderboard")

	// The handler must convert to UTC itself: the season store compares the
	// calendar date of the time it receives, in that time's own zone.
	testutil.AssertEqual(t, "Current asked for", "2026-10-15", store.FormatDate(f.seasons.askedDate))
}

func TestLeaderboardHandler_noMembers(t *testing.T) {
	f := newLeaderboardFixture(t)
	f.standings.rows = []store.Standing{}

	rec := f.get(t, "/api/leaderboard")

	// An empty list, not null: the frontend can always iterate it.
	if !strings.Contains(rec.Body.String(), `"standings":[]`) {
		t.Errorf("body %s: wanted \"standings\":[]", rec.Body.String())
	}
}

func TestLeaderboardHandler_seasonByID(t *testing.T) {
	f := newLeaderboardFixture(t)

	rec := f.get(t, "/api/leaderboard?season=2")

	testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
	testutil.AssertEqual(t, "Get asked for", true, slices.Equal([]int64{2}, f.seasons.askedIDs))
	testutil.AssertEqual(t, "Current called", false, f.seasons.currentCalled)
	testutil.AssertEqual(t, "standings from", "2026-06-01", store.FormatDate(f.standings.from))
	if !strings.Contains(rec.Body.String(), `"name":"Summer 2026"`) {
		t.Errorf("body %s does not show season 2", rec.Body.String())
	}
}

func TestLeaderboardHandler_errors(t *testing.T) {
	testCases := []struct {
		name       string
		path       string
		setup      func(f *leaderboardFixture)
		wantStatus int
		wantStore  bool // whether the stores may be called at all
	}{
		{name: "no current season", path: "/api/leaderboard", setup: func(f *leaderboardFixture) { f.seasons.currentErr = store.ErrNotFound }, wantStatus: http.StatusNotFound, wantStore: true},
		{name: "unknown season", path: "/api/leaderboard?season=99", wantStatus: http.StatusNotFound, wantStore: true},
		{name: "season not a number", path: "/api/leaderboard?season=autumn", wantStatus: http.StatusBadRequest},
		{name: "season zero", path: "/api/leaderboard?season=0", wantStatus: http.StatusBadRequest},
		{name: "season negative", path: "/api/leaderboard?season=-1", wantStatus: http.StatusBadRequest},
		{name: "season with decimals", path: "/api/leaderboard?season=1.5", wantStatus: http.StatusBadRequest},
		{name: "season store fails", path: "/api/leaderboard", setup: func(f *leaderboardFixture) { f.seasons.currentErr = errStoreDown }, wantStatus: http.StatusInternalServerError, wantStore: true},
		{name: "standings store fails", path: "/api/leaderboard", setup: func(f *leaderboardFixture) { f.standings.err = errStoreDown }, wantStatus: http.StatusInternalServerError, wantStore: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newLeaderboardFixture(t)
			if tc.setup != nil {
				tc.setup(f)
			}

			rec := f.get(t, tc.path)

			testutil.AssertEqual(t, "status", tc.wantStatus, rec.Code)
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["error"] == "" {
				t.Errorf("body %q is not a JSON error", rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), errStoreDown.Error()) {
				t.Errorf("body %q leaks the internal error", rec.Body.String())
			}
			if !tc.wantStore && (f.seasons.currentCalled || len(f.seasons.askedIDs) > 0) {
				t.Error("an invalid parameter reached the season store")
			}
		})
	}
}

var errStoreDown = errors.New("database is locked")

type leaderboardFixture struct {
	seasons   *fakeSeasons
	standings *fakeStandings
	now       time.Time
}

func newLeaderboardFixture(t *testing.T) *leaderboardFixture {
	t.Helper()
	autumn := store.Season{ID: 3, Name: "Autumn 2026", StartDate: day(t, "2026-09-01"), EndDate: day(t, "2026-12-31")}
	summer := store.Season{ID: 2, Name: "Summer 2026", StartDate: day(t, "2026-06-01"), EndDate: day(t, "2026-08-31")}
	return &leaderboardFixture{
		seasons: &fakeSeasons{current: autumn, byID: map[int64]store.Season{2: summer, 3: autumn}},
		standings: &fakeStandings{rows: []store.Standing{
			{UserID: 7, FirstName: "Ada", LastName: "Lovelace", Points: 24},
			{UserID: 9, FirstName: "Alan", LastName: "Turing", Points: 18},
			{UserID: 4, FirstName: "Grace", LastName: "Hopper", Points: 18},
		}},
		now: time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC),
	}
}

func (f *leaderboardFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	h := leaderboardHandler(f.seasons, f.standings, func() time.Time { return f.now }, slog.New(slog.DiscardHandler))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

type fakeSeasons struct {
	current       store.Season
	currentErr    error
	byID          map[int64]store.Season
	currentCalled bool
	askedDate     time.Time
	askedIDs      []int64
}

func (f *fakeSeasons) Current(_ context.Context, date time.Time) (store.Season, error) {
	f.currentCalled = true
	f.askedDate = date
	if f.currentErr != nil {
		return store.Season{}, f.currentErr
	}
	return f.current, nil
}

func (f *fakeSeasons) Get(_ context.Context, id int64) (store.Season, error) {
	f.askedIDs = append(f.askedIDs, id)
	s, ok := f.byID[id]
	if !ok {
		return store.Season{}, store.ErrNotFound
	}
	return s, nil
}

type fakeStandings struct {
	rows     []store.Standing
	err      error
	from, to time.Time
}

func (f *fakeStandings) Standings(_ context.Context, from, to time.Time) ([]store.Standing, error) {
	f.from, f.to = from, to
	if f.err != nil {
		return nil, f.err
	}
	return f.rows, nil
}

func day(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := store.ParseDate(s)
	if err != nil {
		t.Fatalf("parse date %q: %v", s, err)
	}
	return d
}

// assertJSON compares JSON documents by value, ignoring formatting.
func assertJSON(t *testing.T, got, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(got), &g); err != nil {
		t.Fatalf("response %q is not JSON: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("expected JSON is invalid: %v", err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("JSON:\n got  %s\n want %s", got, want)
	}
}
