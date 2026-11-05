package backfill

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
	"github.com/mario595/leaderboard/internal/testutil"
)

const seasonID = int64(3)

// testNow is the runner's clock in these tests.
var testNow = time.Date(2026, 10, 15, 12, 7, 30, 0, time.UTC)

// A job imports the season for the job's user, with that user's tokens.
func TestRunner_process(t *testing.T) {
	f := newRunnerFixture(t)
	f.client.pages = map[int][]strava.Activity{1: {
		summary(1, "Walk", "2026-09-15T10:00:00", time.Hour),
		summary(2, "Walk", "2026-10-01T10:00:00", time.Hour),
	}}

	f.runner.handle(t.Context(), job(1, 0))

	testutil.AssertEqual(t, "done", true, f.jobs.wasDone(1))
	assertIDs(t, "saved", []int64{1, 2}, f.saver.ids())
	testutil.AssertEqual(t, "saved for the job's user", userID, f.saver.saved[0].UserID)
	if len(f.built) != 1 {
		t.Fatalf("clients built = %d, want 1", len(f.built))
	}
	testutil.AssertEqual(t, "client user", userID, f.built[0].userID)
	testutil.AssertEqual(t, "client access token", "stored-access", f.built[0].tok.AccessToken)
	testutil.AssertEqual(t, "client refresh token", "stored-refresh", f.built[0].tok.RefreshToken)
	testutil.AssertTime(t, "client token expiry", f.user.TokenExpiresAt, f.built[0].tok.Expiry)
	// The season's range, padded (the details are Import's tests).
	testutil.AssertTime(t, "list after", time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), f.client.lists[0].after)
	testutil.AssertTime(t, "list before", time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC), f.client.lists[0].before)
}

func TestRunner_handle(t *testing.T) {
	rateLimited := &strava.RateLimitError{ShortUsage: 201, ShortLimit: 200, DailyUsage: 900, DailyLimit: 2000}
	dbErr := errors.New("database is locked")

	tests := []struct {
		name        string
		attempts    int
		setup       func(f *runnerFixture)
		wantDone    bool
		wantFailed  bool
		wantRetryAt time.Time // zero: no retry
		wantErrText string    // in last_error, for retries and failures
		noClient    bool      // done before talking to Strava
	}{
		{
			// Deleted (deauthorized) since the job was queued: nothing to do.
			name:     "user gone: done",
			setup:    func(f *runnerFixture) { f.users.err = store.ErrNotFound },
			wantDone: true,
			noClient: true,
		},
		{
			name:     "season gone: done",
			setup:    func(f *runnerFixture) { f.seasons.getErr = store.ErrNotFound },
			wantDone: true,
			noClient: true,
		},
		{
			name:        "rate limited: retry at the next window",
			setup:       func(f *runnerFixture) { f.client.listErr = map[int]error{1: rateLimited} },
			wantRetryAt: time.Date(2026, 10, 15, 12, 15, 0, 0, time.UTC),
			wantErrText: "rate limited",
		},
		{
			// Rate limits do not use up attempts: even the last one waits.
			name:        "rate limited on the last attempt: retry",
			attempts:    4,
			setup:       func(f *runnerFixture) { f.client.listErr = map[int]error{1: rateLimited} },
			wantRetryAt: time.Date(2026, 10, 15, 12, 15, 0, 0, time.UTC),
			wantErrText: "rate limited",
		},
		{
			// Access revoked: retrying cannot help.
			name:        "unauthorized: failed",
			setup:       func(f *runnerFixture) { f.client.listErr = map[int]error{1: strava.ErrUnauthorized} },
			wantFailed:  true,
			wantErrText: "unauthorized",
		},
		{
			name:        "other error: retry in a minute",
			setup:       func(f *runnerFixture) { f.saver.err = dbErr },
			wantRetryAt: testNow.Add(time.Minute),
			wantErrText: "database is locked",
		},
		{
			name:        "other error after 3 failures: retry in 8 minutes",
			attempts:    3,
			setup:       func(f *runnerFixture) { f.saver.err = dbErr },
			wantRetryAt: testNow.Add(8 * time.Minute),
			wantErrText: "database is locked",
		},
		{
			// The fifth failure is the last.
			name:        "other error on the last attempt: failed",
			attempts:    4,
			setup:       func(f *runnerFixture) { f.saver.err = dbErr },
			wantFailed:  true,
			wantErrText: "database is locked",
		},
		{
			// Not "user gone": the database failed, so try again.
			name:        "user lookup fails: retry",
			setup:       func(f *runnerFixture) { f.users.err = dbErr },
			wantRetryAt: testNow.Add(time.Minute),
			wantErrText: "database is locked",
		},
		{
			name:        "season lookup fails: retry",
			setup:       func(f *runnerFixture) { f.seasons.getErr = dbErr },
			wantRetryAt: testNow.Add(time.Minute),
			wantErrText: "database is locked",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newRunnerFixture(t)
			f.client.pages = map[int][]strava.Activity{1: {summary(1, "Walk", "2026-10-01T10:00:00", time.Hour)}}
			tc.setup(f)

			f.runner.handle(t.Context(), job(1, tc.attempts))

			q := f.jobs
			q.mu.Lock()
			defer q.mu.Unlock()
			testutil.AssertEqual(t, "done", tc.wantDone, len(q.done) == 1)
			_, failed := q.failed[1]
			testutil.AssertEqual(t, "failed", tc.wantFailed, failed)
			retryAt, retried := q.retried[1]
			testutil.AssertEqual(t, "retried", !tc.wantRetryAt.IsZero(), retried)
			if retried && !retryAt.Equal(tc.wantRetryAt) {
				t.Errorf("retry at: got %v, want %v", retryAt, tc.wantRetryAt)
			}
			if marks := len(q.done) + len(q.failed) + len(q.retried); marks != 1 {
				t.Errorf("job marked %d times, want exactly once", marks)
			}
			if tc.wantErrText != "" {
				msg := q.failed[1] + q.errMsgs[1]
				if !strings.Contains(msg, tc.wantErrText) {
					t.Errorf("last_error %q does not contain %q", msg, tc.wantErrText)
				}
			}
			if tc.noClient && len(f.built) != 0 {
				t.Errorf("a client was built for a job with nothing to do")
			}
		})
	}
}

func TestRunner_signedUp(t *testing.T) {
	autumn := store.Season{ID: seasonID, Name: "Autumn", StartDate: seasonStart, EndDate: seasonEnd}
	user := store.User{ID: userID, StravaID: 4242}

	t.Run("season covering today: queued", func(t *testing.T) {
		f := newRunnerFixture(t)
		f.seasons.current = &autumn

		if err := f.runner.SignedUp(t.Context(), user); err != nil {
			t.Fatalf("SignedUp: %v", err)
		}

		testutil.AssertEqual(t, "enqueued", 1, len(f.jobs.enqueued))
		testutil.AssertEqual(t, "enqueued user", userID, f.jobs.enqueued[0].UserID)
		testutil.AssertEqual(t, "enqueued season", seasonID, f.jobs.enqueued[0].SeasonID)
	})
	t.Run("no season today: nothing queued", func(t *testing.T) {
		f := newRunnerFixture(t)

		if err := f.runner.SignedUp(t.Context(), user); err != nil {
			t.Fatalf("SignedUp without a season: %v, want nil", err)
		}

		testutil.AssertEqual(t, "enqueued", 0, len(f.jobs.enqueued))
	})
	t.Run("today is the UTC date", func(t *testing.T) {
		f := newRunnerFixture(t)
		f.seasons.current = &autumn
		// 23:30 on Dec 31 in New York is already Jan 1 in UTC.
		f.runner.now = func() time.Time {
			return time.Date(2026, 12, 31, 23, 30, 0, 0, time.FixedZone("EST", -5*3600))
		}

		if err := f.runner.SignedUp(t.Context(), user); err != nil {
			t.Fatalf("SignedUp: %v", err)
		}

		testutil.AssertEqual(t, "date asked for", "2027-01-01", f.seasons.currentDate.Format(time.DateOnly))
	})
	t.Run("season lookup fails", func(t *testing.T) {
		f := newRunnerFixture(t)
		f.seasons.currentErr = errors.New("database is locked")

		if err := f.runner.SignedUp(t.Context(), user); err == nil {
			t.Error("SignedUp with a failing season lookup: got nil error")
		}
		testutil.AssertEqual(t, "enqueued", 0, len(f.jobs.enqueued))
	})
	t.Run("enqueue fails", func(t *testing.T) {
		f := newRunnerFixture(t)
		f.seasons.current = &autumn
		f.jobs.enqueueErr = errors.New("database is locked")

		if err := f.runner.SignedUp(t.Context(), user); err == nil {
			t.Error("SignedUp with a failing enqueue: got nil error")
		}
	})
	t.Run("never blocks without a running runner", func(t *testing.T) {
		f := newRunnerFixture(t)
		f.seasons.current = &autumn

		done := make(chan struct{})
		go func() {
			defer close(done)
			for range 3 { // more wakes than the channel holds
				_ = f.runner.SignedUp(t.Context(), user)
			}
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("SignedUp blocked: the wake must not wait for the runner")
		}
	})
}

// Jobs left from before a restart run at start; a sign-up wakes the runner
// for its new job without waiting for the poll.
func TestRunner_run(t *testing.T) {
	f := newRunnerFixture(t)
	f.runner.pollInterval = time.Hour // only the start and the wake can trigger work
	f.seasons.current = &store.Season{ID: seasonID, StartDate: seasonStart, EndDate: seasonEnd}
	f.jobs.add(job(1, 0))
	f.jobs.add(job(2, 0))

	stop := startRunner(t, f.runner)
	defer stop()
	waitFor(t, "the 2 waiting jobs done", func() bool { return f.jobs.doneCount() == 2 })

	if err := f.runner.SignedUp(t.Context(), f.user); err != nil {
		t.Fatalf("SignedUp: %v", err)
	}
	waitFor(t, "the sign-up's job done after the wake", func() bool { return f.jobs.doneCount() == 3 })
}

func TestRunner_cancelReturnsPromptly(t *testing.T) {
	f := newRunnerFixture(t)
	f.runner.pollInterval = time.Hour

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- f.runner.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run after cancel: got error %v, want nil (shutdown is not an error)", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return within 1s of cancellation")
	}
}

// runnerFixture is a Runner wired to fakes. The fake client serves no
// activities unless a test sets its pages.
type runnerFixture struct {
	runner  *Runner
	user    store.User
	jobs    *fakeJobs
	users   *fakeUsers
	seasons *fakeSeasons
	saver   *fakeSaver
	client  *fakeClient
	built   []builtClient // written by the runner's goroutine; read after it stops
}

type builtClient struct {
	userID int64
	tok    oauth2.Token
}

func newRunnerFixture(t *testing.T) *runnerFixture {
	t.Helper()
	f := &runnerFixture{
		user: store.User{
			ID:             userID,
			StravaID:       4242,
			AccessToken:    "stored-access",
			RefreshToken:   "stored-refresh",
			TokenExpiresAt: testNow.Add(time.Hour),
		},
		jobs:    &fakeJobs{failed: map[int64]string{}, retried: map[int64]time.Time{}, errMsgs: map[int64]string{}},
		seasons: &fakeSeasons{season: store.Season{ID: seasonID, Name: "Autumn", StartDate: seasonStart, EndDate: seasonEnd}},
		saver:   &fakeSaver{},
		client:  &fakeClient{},
	}
	f.users = &fakeUsers{user: f.user}
	newClient := func(userID int64, tok *oauth2.Token) Client {
		f.built = append(f.built, builtClient{userID: userID, tok: *tok})
		return f.client
	}
	f.runner = NewRunner(f.jobs, f.users, f.seasons, f.saver, newClient, slog.New(slog.DiscardHandler))
	f.runner.now = func() time.Time { return testNow }
	return f
}

func job(id int64, attempts int) store.BackfillJob {
	return store.BackfillJob{ID: id, UserID: userID, SeasonID: seasonID, Status: "pending", Attempts: attempts}
}

// fakeJobs is the queue. Run reads it from its own goroutine, so every
// method locks.
type fakeJobs struct {
	mu         sync.Mutex
	pending    []store.BackfillJob
	nextID     int64
	enqueued   []store.BackfillJob
	enqueueErr error
	done       []int64
	failed     map[int64]string    // id -> error message
	retried    map[int64]time.Time // id -> not before
	errMsgs    map[int64]string    // id -> error message of the retry
}

func (q *fakeJobs) add(j store.BackfillJob) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pending = append(q.pending, j)
	q.nextID = max(q.nextID, j.ID)
}

func (q *fakeJobs) Enqueue(_ context.Context, userID, seasonID int64) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.enqueueErr != nil {
		return false, q.enqueueErr
	}
	q.nextID++
	j := store.BackfillJob{ID: q.nextID, UserID: userID, SeasonID: seasonID, Status: "pending"}
	q.enqueued = append(q.enqueued, j)
	q.pending = append(q.pending, j)
	return true, nil
}

func (q *fakeJobs) NextDue(_ context.Context) (store.BackfillJob, bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.pending) == 0 {
		return store.BackfillJob{}, false, nil
	}
	j := q.pending[0]
	q.pending = q.pending[1:]
	return j, true, nil
}

func (q *fakeJobs) MarkDone(_ context.Context, id int64) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.done = append(q.done, id)
	return nil
}

func (q *fakeJobs) MarkRetry(_ context.Context, id int64, errMsg string, notBefore time.Time) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.retried[id] = notBefore
	q.errMsgs[id] = errMsg
	return nil
}

func (q *fakeJobs) MarkFailed(_ context.Context, id int64, errMsg string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.failed[id] = errMsg
	return nil
}

func (q *fakeJobs) wasDone(id int64) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, d := range q.done {
		if d == id {
			return true
		}
	}
	return false
}

func (q *fakeJobs) doneCount() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.done)
}

type fakeUsers struct {
	user store.User
	err  error
}

func (f *fakeUsers) GetByID(_ context.Context, id int64) (store.User, error) {
	if f.err != nil {
		return store.User{}, f.err
	}
	if id != f.user.ID {
		return store.User{}, store.ErrNotFound
	}
	return f.user, nil
}

// fakeSeasons has one season for Get, and current (nil: none) for Current.
type fakeSeasons struct {
	season      store.Season
	getErr      error
	current     *store.Season
	currentErr  error
	currentDate time.Time // the date Current was asked for
}

func (f *fakeSeasons) Get(_ context.Context, id int64) (store.Season, error) {
	if f.getErr != nil {
		return store.Season{}, f.getErr
	}
	if id != f.season.ID {
		return store.Season{}, store.ErrNotFound
	}
	return f.season, nil
}

func (f *fakeSeasons) Current(_ context.Context, date time.Time) (store.Season, error) {
	f.currentDate = date
	if f.currentErr != nil {
		return store.Season{}, f.currentErr
	}
	if f.current == nil {
		return store.Season{}, store.ErrNotFound
	}
	return *f.current, nil
}

// startRunner runs r until the returned stop is called (or the test ends).
func startRunner(t *testing.T, r *Runner) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = r.Run(ctx)
	}()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
	t.Cleanup(stop)
	return stop
}

// waitFor polls cond until it holds, failing the test after 2s.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// The real types satisfy the interfaces.
var (
	_ JobQueue = (*store.BackfillStore)(nil)
	_ Users    = (*store.UserStore)(nil)
	_ Seasons  = (*store.SeasonStore)(nil)
)
