package reconcile

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/activity"
	"github.com/mario595/leaderboard/internal/backfill"
	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
	"github.com/mario595/leaderboard/internal/testutil"
)

const userID = int64(7)

// The window of most tests: one week, both days inclusive.
var (
	from = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to   = time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
)

// What Strava lists is saved; what is stored but Strava no longer lists is
// deleted. The stored IDs are read for the same user and window.
func TestUser_savesAndDeletes(t *testing.T) {
	client := &fakeClient{list: []strava.Activity{
		summary(1, "Walk", "2026-10-01T10:00:00"), // stored, still there
		summary(3, "Walk", "2026-10-03T10:00:00"), // stored, still there
		// Listed, but deleted before its detail could be fetched: not saved,
		// so deleted too.
		summary(4, "Run", "2026-10-04T07:00:00"),
		summary(5, "Walk", "2026-10-05T10:00:00"), // missed create
	}}
	acts := &fakeActivities{stored: []int64{1, 2, 3, 4}} // 2: missed delete

	res, err := User(t.Context(), client, acts, discard, userID, from, to)
	if err != nil {
		t.Fatalf("User: %v", err)
	}

	assertIDs(t, "saved", []int64{1, 3, 5}, acts.savedIDs())
	assertIDs(t, "deleted", []int64{2, 4}, acts.deleted)
	testutil.AssertEqual(t, "Result.Saved", 3, res.Saved)
	testutil.AssertEqual(t, "Result.Deleted", 2, res.Deleted)

	testutil.AssertEqual(t, "IDsBetween calls", 1, len(acts.idsCalls))
	c := acts.idsCalls[0]
	testutil.AssertEqual(t, "IDsBetween userID", userID, c.userID)
	testutil.AssertTime(t, "IDsBetween from", from, c.from)
	testutil.AssertTime(t, "IDsBetween to", to, c.to)
	// Saved for the right user.
	testutil.AssertEqual(t, "saved UserID", userID, acts.saved[0].UserID)
}

// Positive control: when the store and Strava agree, nothing is deleted.
func TestUser_nothingToDelete(t *testing.T) {
	client := &fakeClient{list: []strava.Activity{
		summary(1, "Walk", "2026-10-01T10:00:00"),
		summary(2, "Walk", "2026-10-02T10:00:00"),
	}}
	acts := &fakeActivities{stored: []int64{1, 2}}

	res, err := User(t.Context(), client, acts, discard, userID, from, to)
	if err != nil {
		t.Fatalf("User: %v", err)
	}

	testutil.AssertEqual(t, "deletes", 0, len(acts.deleted))
	testutil.AssertEqual(t, "Result.Saved", 2, res.Saved)
	testutil.AssertEqual(t, "Result.Deleted", 0, res.Deleted)
}

// A failed import has a partial list: deleting what it missed would delete
// real activities. The error keeps its type, for the log.
func TestUser_importFails(t *testing.T) {
	client := &fakeClient{listErr: &strava.RateLimitError{ShortUsage: 201, ShortLimit: 200}}
	acts := &fakeActivities{stored: []int64{1, 2}}

	_, err := User(t.Context(), client, acts, discard, userID, from, to)

	var rl *strava.RateLimitError
	if !errors.As(err, &rl) {
		t.Errorf("User: error %v, want a *strava.RateLimitError", err)
	}
	testutil.AssertEqual(t, "deletes", 0, len(acts.deleted))
}

func TestUser_storeFails(t *testing.T) {
	list := []strava.Activity{summary(1, "Walk", "2026-10-01T10:00:00")}

	t.Run("stored IDs", func(t *testing.T) {
		acts := &fakeActivities{stored: []int64{1, 2}, idsErr: errors.New("database is locked")}

		_, err := User(t.Context(), &fakeClient{list: list}, acts, discard, userID, from, to)

		if err == nil {
			t.Fatal("User with a failing IDsBetween: got nil error")
		}
		testutil.AssertEqual(t, "deletes", 0, len(acts.deleted))
	})
	t.Run("delete", func(t *testing.T) {
		acts := &fakeActivities{stored: []int64{1, 2, 3}, deleteErr: errors.New("database is locked")}

		_, err := User(t.Context(), &fakeClient{list: list}, acts, discard, userID, from, to)

		if err == nil {
			t.Fatal("User with a failing Delete: got nil error")
		}
		// Stops at the first failure: the next run tries again.
		testutil.AssertEqual(t, "delete attempts", 1, acts.deleteCalls)
	})
}

// discard is the logger of tests that do not check logs.
var discard = slog.New(slog.DiscardHandler)

// summary returns an activity as the list returns it: no description, an
// hour long.
func summary(id int64, sport, startLocal string) strava.Activity {
	start, err := time.Parse("2006-01-02T15:04:05", startLocal)
	if err != nil {
		panic(err)
	}
	return strava.Activity{ID: id, SportType: sport, StartLocal: start, MovingTime: time.Hour}
}

// fakeClient lists one page and knows no details: a detail fetch is
// strava.ErrNotFound.
type fakeClient struct {
	list    []strava.Activity
	listErr error
}

func (f *fakeClient) ListActivities(_ context.Context, _, _ time.Time, _, _ int) ([]strava.Activity, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.list, nil
}

func (f *fakeClient) Activity(_ context.Context, _ int64) (strava.Activity, error) {
	return strava.Activity{}, strava.ErrNotFound
}

type idsCall struct {
	userID   int64
	from, to time.Time
}

// fakeActivities records saves and deletes, and returns stored as the stored
// IDs.
type fakeActivities struct {
	stored    []int64
	idsErr    error
	deleteErr error

	saved       []store.Activity
	deleted     []int64
	deleteCalls int
	idsCalls    []idsCall
}

func (f *fakeActivities) Save(_ context.Context, a store.Activity) error {
	f.saved = append(f.saved, a)
	return nil
}

func (f *fakeActivities) Delete(_ context.Context, stravaID int64) error {
	f.deleteCalls++
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, stravaID)
	return nil
}

func (f *fakeActivities) IDsBetween(_ context.Context, userID int64, from, to time.Time) ([]int64, error) {
	f.idsCalls = append(f.idsCalls, idsCall{userID: userID, from: from, to: to})
	if f.idsErr != nil {
		return nil, f.idsErr
	}
	return f.stored, nil
}

func (f *fakeActivities) savedIDs() []int64 {
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
	_ Activities      = (*activity.Service)(nil)
	_ backfill.Client = (*strava.Client)(nil)
)
