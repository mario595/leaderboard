package backfill

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/activity"
	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
	"github.com/mario595/leaderboard/internal/testutil"
)

const userID = int64(7)

// The season of most tests: autumn 2026, both days inclusive.
var (
	seasonStart = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	seasonEnd   = time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
)

// Every activity in the season is saved for the user. The detail (with the
// description) is fetched only when the #competition tag could add points:
// a counted sport, long enough, and not already a race.
func TestImport_savesTheSeasonsActivities(t *testing.T) {
	run := summary(1, "Run", "2026-09-15T07:00:00", 45*time.Minute)
	run.Distance = 10000
	raceRide := summary(2, "GravelRide", "2026-10-01T09:00:00", 3*time.Hour)
	raceRide.WorkoutType = 11                                            // a Race already scores 4
	walk := summary(3, "Walk", "2026-10-02T10:00:00", 2*time.Hour)       // not counted
	shortRun := summary(4, "Run", "2026-10-03T07:00:00", 29*time.Minute) // too short
	swim := summary(5, "Swim", "2026-10-04T19:00:00", 45*time.Minute)
	eBike := summary(6, "EBikeRide", "2026-10-05T19:00:00", 2*time.Hour) // not counted

	runDetail := run
	runDetail.Description = "10k road race #competition"
	swimDetail := swim
	swimDetail.Description = "pool"
	client := &fakeClient{
		pages:   map[int][]strava.Activity{1: {run, raceRide, walk, shortRun, swim, eBike}},
		details: map[int64]strava.Activity{1: runDetail, 5: swimDetail},
	}
	saver := &fakeSaver{}

	res, err := Import(t.Context(), client, saver, discard, userID, seasonStart, seasonEnd)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	testutil.AssertEqual(t, "saved count", 6, res.Saved)
	testutil.AssertEqual(t, "detail count", 2, res.Details)
	assertIDs(t, "saved IDs", []int64{1, 2, 3, 4, 5, 6}, saver.ids())
	assertIDs(t, "detail fetches", []int64{1, 5}, client.detailIDs)

	got := saver.saved[0]
	testutil.AssertEqual(t, "UserID", userID, got.UserID)
	testutil.AssertEqual(t, "StravaID", int64(1), got.StravaID)
	testutil.AssertEqual(t, "SportType", "Run", got.SportType)
	testutil.AssertTime(t, "StartLocal", run.StartLocal, got.StartLocal)
	testutil.AssertEqual(t, "MovingTime", 45*time.Minute, got.MovingTime)
	testutil.AssertEqual(t, "Distance", 10000.0, got.Distance)
	// From the detail: this is what makes it a competition.
	testutil.AssertEqual(t, "Description", "10k road race #competition", got.Description)
	// Saved from the summary.
	testutil.AssertEqual(t, "race WorkoutType", 11, saver.saved[1].WorkoutType)
	testutil.AssertEqual(t, "race Description", "", saver.saved[1].Description)
}

// Strava filters by UTC start time, points count by local day: the query is
// padded by a day on each side (UTC offsets go from -12h to +14h).
func TestImport_queryWindow(t *testing.T) {
	client := &fakeClient{}

	if _, err := Import(t.Context(), client, &fakeSaver{}, discard, userID, seasonStart, seasonEnd); err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(client.lists) != 1 {
		t.Fatalf("list calls = %d, want 1 (an empty first page ends the import)", len(client.lists))
	}
	l := client.lists[0]
	testutil.AssertTime(t, "after", time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), l.after)
	// The end of the last day (Jan 1 00:00) plus one day.
	testutil.AssertTime(t, "before", time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC), l.before)
	testutil.AssertEqual(t, "page", 1, l.page)
	testutil.AssertEqual(t, "per_page", 200, l.perPage)
}

// The padding brings in activities of the days around the season: only the
// local day decides.
func TestImport_keepsOnlyTheSeasonsDays(t *testing.T) {
	client := &fakeClient{pages: map[int][]strava.Activity{1: {
		summary(1, "Walk", "2026-08-31T23:59:59", time.Hour), // the day before
		summary(2, "Walk", "2026-09-01T00:00:00", time.Hour), // first day
		summary(3, "Walk", "2026-12-31T23:59:59", time.Hour), // last day
		summary(4, "Walk", "2027-01-01T00:00:00", time.Hour), // the day after
	}}}
	saver := &fakeSaver{}

	res, err := Import(t.Context(), client, saver, discard, userID, seasonStart, seasonEnd)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	testutil.AssertEqual(t, "saved count", 2, res.Saved)
	assertIDs(t, "saved IDs", []int64{2, 3}, saver.ids())
}

// Outside the season, no detail is fetched either: it would waste the budget.
func TestImport_noDetailOutsideTheSeason(t *testing.T) {
	client := &fakeClient{pages: map[int][]strava.Activity{1: {
		summary(1, "Run", "2026-08-31T19:00:00", time.Hour),
	}}}

	if _, err := Import(t.Context(), client, &fakeSaver{}, discard, userID, seasonStart, seasonEnd); err != nil {
		t.Fatalf("Import: %v", err)
	}

	testutil.AssertEqual(t, "detail fetches", 0, len(client.detailIDs))
}

func TestImport_pages(t *testing.T) {
	tests := []struct {
		name      string
		pageSizes []int // activities on pages 1, 2, ...
		wantLists int
	}{
		// A short page is the last one: no request for the next.
		{name: "one short page", pageSizes: []int{3}, wantLists: 1},
		{name: "full page, then a short one", pageSizes: []int{200, 3}, wantLists: 2},
		// A full last page cannot be told apart: one more, empty, request.
		{name: "full page, then nothing", pageSizes: []int{200}, wantLists: 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &fakeClient{pages: map[int][]strava.Activity{}}
			id, total := int64(1), 0
			for i, size := range tc.pageSizes {
				for range size {
					client.pages[i+1] = append(client.pages[i+1], summary(id, "Walk", "2026-10-01T10:00:00", time.Hour))
					id++
				}
				total += size
			}
			saver := &fakeSaver{}

			res, err := Import(t.Context(), client, saver, discard, userID, seasonStart, seasonEnd)
			if err != nil {
				t.Fatalf("Import: %v", err)
			}

			testutil.AssertEqual(t, "list calls", tc.wantLists, len(client.lists))
			for i, l := range client.lists {
				testutil.AssertEqual(t, "page number", i+1, l.page)
			}
			testutil.AssertEqual(t, "saved count", total, res.Saved)
			testutil.AssertEqual(t, "saved", total, len(saver.saved))
		})
	}
}

// An activity deleted between the list and the detail fetch is skipped, not
// an error: retrying would never find it.
func TestImport_activityDeletedMeanwhile(t *testing.T) {
	client := &fakeClient{pages: map[int][]strava.Activity{1: {
		summary(1, "Run", "2026-10-01T07:00:00", time.Hour),
		summary(2, "Run", "2026-10-02T07:00:00", time.Hour),
	}}, details: map[int64]strava.Activity{2: summary(2, "Run", "2026-10-02T07:00:00", time.Hour)}}
	saver := &fakeSaver{}

	res, err := Import(t.Context(), client, saver, discard, userID, seasonStart, seasonEnd)
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	testutil.AssertEqual(t, "saved count", 1, res.Saved)
	assertIDs(t, "saved IDs", []int64{2}, saver.ids())
}

// Any other failure stops the import, with the error's type kept: the
// runner retries a rate limit at the next window.
func TestImport_errorsStopTheImport(t *testing.T) {
	rateLimited := &strava.RateLimitError{ShortUsage: 201, ShortLimit: 200}
	full := func() []strava.Activity {
		var page []strava.Activity
		for i := range 200 {
			page = append(page, summary(int64(100+i), "Walk", "2026-10-01T10:00:00", time.Hour))
		}
		return page
	}

	t.Run("list fails on a later page", func(t *testing.T) {
		client := &fakeClient{pages: map[int][]strava.Activity{1: full()}, listErr: map[int]error{2: rateLimited}}

		_, err := Import(t.Context(), client, &fakeSaver{}, discard, userID, seasonStart, seasonEnd)

		var rl *strava.RateLimitError
		if !errors.As(err, &rl) {
			t.Errorf("Import: error %v, want a *strava.RateLimitError", err)
		}
	})
	t.Run("detail fails", func(t *testing.T) {
		client := &fakeClient{
			pages:     map[int][]strava.Activity{1: {summary(1, "Run", "2026-10-01T07:00:00", time.Hour), summary(2, "Walk", "2026-10-02T07:00:00", time.Hour)}},
			detailErr: rateLimited,
		}
		saver := &fakeSaver{}

		_, err := Import(t.Context(), client, saver, discard, userID, seasonStart, seasonEnd)

		var rl *strava.RateLimitError
		if !errors.As(err, &rl) {
			t.Errorf("Import: error %v, want a *strava.RateLimitError", err)
		}
		// Not saved from the summary instead: that would lose the tag.
		testutil.AssertEqual(t, "saved", 0, len(saver.saved))
	})
	t.Run("save fails", func(t *testing.T) {
		client := &fakeClient{pages: map[int][]strava.Activity{1: {
			summary(1, "Walk", "2026-10-01T07:00:00", time.Hour),
			summary(2, "Walk", "2026-10-02T07:00:00", time.Hour),
		}}}
		saver := &fakeSaver{err: errors.New("database is locked")}

		_, err := Import(t.Context(), client, saver, discard, userID, seasonStart, seasonEnd)

		if err == nil {
			t.Fatal("Import with a failing save: got nil error")
		}
		testutil.AssertEqual(t, "save attempts", 1, saver.calls)
	})
}

// discard is the logger of tests that do not check logs.
var discard = slog.New(slog.DiscardHandler)

// summary returns an activity as the list returns it: no description.
func summary(id int64, sport, startLocal string, moving time.Duration) strava.Activity {
	start, err := time.Parse("2006-01-02T15:04:05", startLocal)
	if err != nil {
		panic(err)
	}
	return strava.Activity{ID: id, SportType: sport, StartLocal: start, MovingTime: moving}
}

type listCall struct {
	after, before time.Time
	page, perPage int
}

// fakeClient serves pages of summaries and details by ID. A detail it does
// not know is strava.ErrNotFound.
type fakeClient struct {
	pages     map[int][]strava.Activity
	details   map[int64]strava.Activity
	listErr   map[int]error // by page
	detailErr error
	// listing, when set, makes ListActivities report that it started and then
	// wait for its context to end, as a request waiting for the rate limiter
	// does.
	listing chan struct{}

	lists     []listCall
	detailIDs []int64
}

func (f *fakeClient) ListActivities(ctx context.Context, after, before time.Time, page, perPage int) ([]strava.Activity, error) {
	f.lists = append(f.lists, listCall{after: after, before: before, page: page, perPage: perPage})
	if f.listing != nil {
		f.listing <- struct{}{}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if err := f.listErr[page]; err != nil {
		return nil, err
	}
	return f.pages[page], nil
}

func (f *fakeClient) Activity(_ context.Context, id int64) (strava.Activity, error) {
	f.detailIDs = append(f.detailIDs, id)
	if f.detailErr != nil {
		return strava.Activity{}, f.detailErr
	}
	a, ok := f.details[id]
	if !ok {
		return strava.Activity{}, strava.ErrNotFound
	}
	return a, nil
}

type fakeSaver struct {
	saved []store.Activity
	calls int
	err   error
}

func (f *fakeSaver) Save(_ context.Context, a store.Activity) error {
	f.calls++
	if f.err != nil {
		return f.err
	}
	f.saved = append(f.saved, a)
	return nil
}

func (f *fakeSaver) ids() []int64 {
	var ids []int64
	for _, a := range f.saved {
		ids = append(ids, a.StravaID)
	}
	return ids
}

// assertIDs compares two lists of IDs, order included.
func assertIDs(t *testing.T, name string, want, got []int64) {
	t.Helper()
	if !slices.Equal(want, got) {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

// The real types satisfy the interfaces.
var (
	_ Client        = (*strava.Client)(nil)
	_ ActivitySaver = (*activity.Service)(nil)
)
