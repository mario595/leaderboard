package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
	"github.com/mario595/leaderboard/internal/testutil"
)

const (
	athleteID  = 134815 // Strava athlete ID of the test user
	internalID = 7      // the test user's ID in our database
	activityID = 12345678901
)

func TestProcess(t *testing.T) {
	ride := strava.Activity{
		ID:          activityID,
		SportType:   "GravelRide",
		StartLocal:  time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC),
		MovingTime:  8102 * time.Second,
		Distance:    61234,
		WorkoutType: 11,
		Description: "#competition",
	}
	rateLimited := &strava.RateLimitError{ShortUsage: 201, ShortLimit: 200, DailyUsage: 900, DailyLimit: 2000}
	errStrava := errors.New("strava GET /activities/12345678901: status 500")

	testCases := []struct {
		name       string
		event      store.Event
		fetches    []fetchResult // what each client's Activity call returns, in order
		saveErr    error
		wantErr    error // nil, or matched with errors.Is
		wantSaved  bool
		wantDelete bool
		wantFetch  int // Activity calls
	}{
		{name: "create: activity found is saved", event: activityEvent("create"), fetches: []fetchResult{{a: ride}}, wantSaved: true, wantFetch: 1},
		{name: "update: activity found is saved", event: activityEvent("update"), fetches: []fetchResult{{a: ride}}, wantSaved: true, wantFetch: 1},
		{name: "delete: gone on Strava is deleted", event: activityEvent("delete"), fetches: []fetchResult{{err: strava.ErrNotFound}}, wantDelete: true, wantFetch: 1},
		{
			// A forged delete, or one overtaken by a later create: Strava has
			// the last word.
			name: "delete: still on Strava is saved, not deleted", event: activityEvent("delete"),
			fetches: []fetchResult{{a: ride}}, wantSaved: true, wantFetch: 1,
		},
		{name: "update: made private (404) is deleted", event: activityEvent("update"), fetches: []fetchResult{{err: strava.ErrNotFound}}, wantDelete: true, wantFetch: 1},
		{
			// No forced refresh: the event fails, and the next event for the
			// activity (or the reconcile job, #31) brings it back.
			name: "401: unauthorized, no second try", event: activityEvent("create"),
			fetches: []fetchResult{{err: strava.ErrUnauthorized}}, wantErr: strava.ErrUnauthorized, wantFetch: 1,
		},
		{name: "rate limited: error, nothing saved", event: activityEvent("create"), fetches: []fetchResult{{err: rateLimited}}, wantErr: rateLimited, wantFetch: 1},
		{name: "other Strava error: returned", event: activityEvent("create"), fetches: []fetchResult{{err: errStrava}}, wantErr: errStrava, wantFetch: 1},
		{name: "save fails: returned", event: activityEvent("create"), fetches: []fetchResult{{a: ride}}, saveErr: errDB, wantErr: errDB, wantSaved: false, wantFetch: 1},
		{
			// #15 handles deauthorization; until then athlete events are done.
			name: "athlete event: ignored", event: store.Event{ID: 1, ObjectType: "athlete", ObjectID: athleteID, AspectType: "update", OwnerID: athleteID, Updates: `{"authorized":"false"}`},
			wantFetch: 0,
		},
		{
			name: "owner is not a user: ignored", event: store.Event{ID: 1, ObjectType: "activity", ObjectID: activityID, AspectType: "create", OwnerID: 999},
			wantFetch: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, tc.fetches)
			f.activities.saveErr = tc.saveErr

			err := f.worker.process(t.Context(), tc.event)

			if tc.wantErr == nil && err != nil {
				t.Fatalf("process: got error %v, wanted nil", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("process: got error %v, wanted one wrapping %v", err, tc.wantErr)
			}
			testutil.AssertEqual(t, "Activity calls", tc.wantFetch, f.clients.fetchCount())

			saved, deleted := f.activities.calls()
			testutil.AssertEqual(t, "saved", tc.wantSaved, len(saved) == 1)
			testutil.AssertEqual(t, "deleted", tc.wantDelete, len(deleted) == 1)
			if len(saved)+len(deleted) > 1 {
				t.Errorf("more than one change: saved %v, deleted %v", saved, deleted)
			}
			if tc.wantDelete {
				testutil.AssertEqual(t, "deleted ID", int64(activityID), deleted[0])
			}
			if tc.wantSaved {
				got := saved[0]
				// Mapping from strava.Activity, plus the owner's internal ID.
				testutil.AssertEqual(t, "UserID", int64(internalID), got.UserID)
				testutil.AssertEqual(t, "StravaID", ride.ID, got.StravaID)
				testutil.AssertEqual(t, "SportType", ride.SportType, got.SportType)
				testutil.AssertTime(t, "StartLocal", ride.StartLocal, got.StartLocal)
				testutil.AssertEqual(t, "MovingTime", ride.MovingTime, got.MovingTime)
				testutil.AssertEqual(t, "Distance", ride.Distance, got.Distance)
				testutil.AssertEqual(t, "WorkoutType", ride.WorkoutType, got.WorkoutType)
				testutil.AssertEqual(t, "Description", ride.Description, got.Description)
			}
		})
	}
}

func TestProcess_clientUsesTheOwnersTokens(t *testing.T) {
	f := newFixture(t, []fetchResult{{err: strava.ErrNotFound}})

	if err := f.worker.process(t.Context(), activityEvent("delete")); err != nil {
		t.Fatalf("process: %v", err)
	}

	built := f.clients.builtWith()
	if len(built) != 1 {
		t.Fatalf("clients built: got %d, wanted 1", len(built))
	}
	// The stored tokens, for the user's internal ID (refreshed tokens are
	// saved under it). The expiry must be the stored one: a zero Expiry
	// means "never expires" and oauth2 would never refresh.
	testutil.AssertEqual(t, "user ID", int64(internalID), built[0].userID)
	testutil.AssertEqual(t, "access token", "stored-access", built[0].tok.AccessToken)
	testutil.AssertEqual(t, "refresh token", "stored-refresh", built[0].tok.RefreshToken)
	testutil.AssertTime(t, "expiry", f.user.TokenExpiresAt, built[0].tok.Expiry)
}

func TestProcess_userLookupError(t *testing.T) {
	f := newFixture(t, nil)
	f.users.err = errDB

	err := f.worker.process(t.Context(), activityEvent("create"))
	if !errors.Is(err, errDB) {
		t.Errorf("process with a failing user store: got error %v, wanted one wrapping %v", err, errDB)
	}
	testutil.AssertEqual(t, "Activity calls", 0, f.clients.fetchCount())
}

var errDB = errors.New("database is locked")

func activityEvent(aspect string) store.Event {
	return store.Event{ID: 1, ObjectType: "activity", ObjectID: activityID, AspectType: aspect, OwnerID: athleteID, Updates: "{}"}
}

// fixture is a Worker wired to fakes.
type fixture struct {
	worker     *Worker
	user       store.User
	users      *fakeUsers
	activities *fakeActivities
	clients    *fakeClients
	events     *fakeQueue
}

func newFixture(t *testing.T, fetches []fetchResult) *fixture {
	t.Helper()
	f := &fixture{
		user: store.User{
			ID:             internalID,
			StravaID:       athleteID,
			AccessToken:    "stored-access",
			RefreshToken:   "stored-refresh",
			TokenExpiresAt: time.Now().Add(time.Hour).Truncate(time.Second),
		},
		activities: &fakeActivities{},
		clients:    &fakeClients{results: fetches},
		events:     &fakeQueue{},
	}
	f.users = &fakeUsers{byStravaID: map[int64]store.User{athleteID: f.user}}
	f.worker = New(f.events, f.users, f.activities, f.clients.newClient, nil, slog.New(slog.DiscardHandler))
	return f
}

type fakeUsers struct {
	byStravaID map[int64]store.User
	err        error
}

func (f *fakeUsers) GetByStravaID(_ context.Context, stravaID int64) (store.User, error) {
	if f.err != nil {
		return store.User{}, f.err
	}
	u, ok := f.byStravaID[stravaID]
	if !ok {
		return store.User{}, store.ErrNotFound
	}
	return u, nil
}

type fakeActivities struct {
	mu      sync.Mutex
	saveErr error
	saved   []store.Activity
	deleted []int64
}

func (f *fakeActivities) Save(_ context.Context, a store.Activity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, a)
	return nil
}

func (f *fakeActivities) Delete(_ context.Context, stravaID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, stravaID)
	return nil
}

func (f *fakeActivities) calls() ([]store.Activity, []int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.Activity(nil), f.saved...), append([]int64(nil), f.deleted...)
}

type fetchResult struct {
	a   strava.Activity
	err error
}

type builtClient struct {
	userID int64
	tok    oauth2.Token
}

// fakeClients builds fake Strava clients. Every Activity call, on any client,
// returns the next result in order.
type fakeClients struct {
	mu      sync.Mutex
	results []fetchResult
	fetches int
	built   []builtClient
}

func (f *fakeClients) newClient(userID int64, tok *oauth2.Token) ActivityFetcher {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.built = append(f.built, builtClient{userID: userID, tok: *tok})
	return fakeClient{f}
}

func (f *fakeClients) fetchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fetches
}

func (f *fakeClients) builtWith() []builtClient {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]builtClient(nil), f.built...)
}

type fakeClient struct{ clients *fakeClients }

func (c fakeClient) Activity(_ context.Context, id int64) (strava.Activity, error) {
	f := c.clients
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fetches >= len(f.results) {
		return strava.Activity{}, errors.New("fakeClient: no result left")
	}
	r := f.results[f.fetches]
	f.fetches++
	return r.a, r.err
}

// fakeQueue is an in-memory EventQueue (used from step 3 on).
type fakeQueue struct {
	mu      sync.Mutex
	pending []store.Event
	done    []int64
	retried map[int64]time.Time
	failed  map[int64]string
	errMsgs map[int64]string
}

func (q *fakeQueue) NextDue(_ context.Context) (store.Event, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return store.Event{}, false, nil
	}
	return q.pending[0], true, nil
}

func (q *fakeQueue) remove(id int64) {
	for i, e := range q.pending {
		if e.ID == id {
			q.pending = append(q.pending[:i], q.pending[i+1:]...)
			return
		}
	}
}

func (q *fakeQueue) MarkDone(_ context.Context, id int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.done = append(q.done, id)
	q.remove(id)
	return nil
}

func (q *fakeQueue) MarkRetry(_ context.Context, id int64, errMsg string, next time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.retried == nil {
		q.retried, q.errMsgs = map[int64]time.Time{}, map[int64]string{}
	}
	q.retried[id] = next
	q.errMsgs[id] = errMsg
	q.remove(id)
	return nil
}

func (q *fakeQueue) MarkFailed(_ context.Context, id int64, errMsg string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.failed == nil {
		q.failed = map[int64]string{}
	}
	q.failed[id] = errMsg
	q.remove(id)
	return nil
}
