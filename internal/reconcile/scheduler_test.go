package reconcile

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/mario595/leaderboard/internal/backfill"
	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
	"github.com/mario595/leaderboard/internal/testutil"
)

// Late on Nov 7 in UTC, already Nov 8 at +03:00: "today" is the UTC date.
var schedulerNow = time.Date(2026, 11, 8, 1, 30, 0, 0, time.FixedZone("+03", 3*60*60))

// Every member is reconciled with their own client, over the last week: from
// 7 days before today (UTC) to today.
func TestScheduler_passReconcilesEveryUser(t *testing.T) {
	f := newSchedulerFixture(t)
	f.clients[1] = &fakeClient{list: []strava.Activity{summary(11, "Walk", "2026-11-02T10:00:00")}}
	f.clients[2] = &fakeClient{list: []strava.Activity{summary(21, "Walk", "2026-11-03T10:00:00")}}

	f.scheduler.pass(t.Context())

	assertIDs(t, "clients built for", []int64{1, 2}, f.builtFor)
	tok := f.tokens[0]
	testutil.AssertEqual(t, "access token", "access-1", tok.AccessToken)
	testutil.AssertEqual(t, "refresh token", "refresh-1", tok.RefreshToken)
	testutil.AssertTime(t, "expiry", f.users.list[0].TokenExpiresAt, tok.Expiry)

	// Each activity is saved for the member whose client listed it.
	testutil.AssertEqual(t, "saves", 2, len(f.acts.saved))
	testutil.AssertEqual(t, "activity 11 UserID", int64(1), f.acts.saved[0].UserID)
	testutil.AssertEqual(t, "activity 21 UserID", int64(2), f.acts.saved[1].UserID)

	testutil.AssertEqual(t, "IDsBetween calls", 2, len(f.acts.idsCalls))
	for _, c := range f.acts.idsCalls {
		testutil.AssertEqual(t, "from", "2026-10-31", c.from.Format(time.DateOnly))
		testutil.AssertEqual(t, "to", "2026-11-07", c.to.Format(time.DateOnly))
	}
}

// A failure for one member is logged and the others go on: the next pass
// retries it.
func TestScheduler_passGoesOnAfterAnError(t *testing.T) {
	f := newSchedulerFixture(t)
	f.clients[1] = &fakeClient{listErr: &strava.RateLimitError{ShortUsage: 201, ShortLimit: 200}}
	f.clients[2] = &fakeClient{list: []strava.Activity{summary(21, "Walk", "2026-11-03T10:00:00")}}

	f.scheduler.pass(t.Context())

	// Member 1's import failed before its stored IDs were read.
	testutil.AssertEqual(t, "IDsBetween calls", 1, len(f.acts.idsCalls))
	testutil.AssertEqual(t, "IDsBetween userID", int64(2), f.acts.idsCalls[0].userID)
	assertLogged(t, f.logs, "level=ERROR", "user_id=1", "rate limit")
	// Only the error: no result line as if member 1 had been reconciled.
	assertNotLogged(t, f.logs, "level=INFO", "user_id=1")
	assertLogged(t, f.logs, "level=INFO", "user_id=2", "saved=1")
}

func TestScheduler_passListFails(t *testing.T) {
	f := newSchedulerFixture(t)
	f.users.err = errors.New("database is locked")

	f.scheduler.pass(t.Context())

	testutil.AssertEqual(t, "clients built", 0, len(f.builtFor))
	assertLogged(t, f.logs, "level=ERROR", "database is locked")
}

// A shutdown stops the pass before the next member, not after all of them.
func TestScheduler_passStopsWhenCancelled(t *testing.T) {
	f := newSchedulerFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.onBuild = cancel // the shutdown arrives during member 1

	f.scheduler.pass(ctx)

	assertIDs(t, "clients built for", []int64{1}, f.builtFor)
}

func TestScheduler_run(t *testing.T) {
	// Run runs a pass at once, without waiting for the first tick.
	t.Run("pass at startup", func(t *testing.T) {
		f := newSchedulerFixture(t)
		f.users.list = nil
		f.scheduler.interval = time.Hour

		stop := f.start(t)
		f.waitForPass(t)
		stop()
	})
	// Then one on every tick.
	t.Run("pass on each tick", func(t *testing.T) {
		f := newSchedulerFixture(t)
		f.users.list = nil
		f.scheduler.interval = 10 * time.Millisecond

		stop := f.start(t)
		for range 3 {
			f.waitForPass(t)
		}
		stop()
	})
}

type schedulerFixture struct {
	scheduler *Scheduler
	users     *fakeUsers
	acts      *fakeActivities
	clients   map[int64]*fakeClient // by user ID; missing = an empty list
	logs      *bytes.Buffer
	onBuild   func() // called after each client is built, when set

	builtFor []int64
	tokens   []*oauth2.Token
}

// newSchedulerFixture returns a scheduler for two members, IDs 1 and 2, with a
// fixed clock.
func newSchedulerFixture(t *testing.T) *schedulerFixture {
	t.Helper()
	f := &schedulerFixture{
		users: &fakeUsers{
			list: []store.User{
				{ID: 1, AccessToken: "access-1", RefreshToken: "refresh-1", TokenExpiresAt: time.Date(2026, 11, 8, 3, 0, 0, 0, time.UTC)},
				{ID: 2, AccessToken: "access-2", RefreshToken: "refresh-2", TokenExpiresAt: time.Date(2026, 11, 8, 4, 0, 0, 0, time.UTC)},
			},
			listed: make(chan struct{}, 1),
		},
		acts:    &fakeActivities{},
		clients: map[int64]*fakeClient{},
		logs:    &bytes.Buffer{},
	}
	newClient := func(userID int64, tok *oauth2.Token) backfill.Client {
		f.builtFor = append(f.builtFor, userID)
		f.tokens = append(f.tokens, tok)
		if f.onBuild != nil {
			f.onBuild()
		}
		if c, ok := f.clients[userID]; ok {
			return c
		}
		return &fakeClient{}
	}
	logger := slog.New(slog.NewTextHandler(f.logs, nil))
	f.scheduler = NewScheduler(f.users, f.acts, newClient, logger)
	f.scheduler.now = func() time.Time { return schedulerNow }
	return f
}

// start runs the scheduler in a goroutine. The returned function cancels it
// and fails the test if Run does not return nil soon.
func (f *schedulerFixture) start(t *testing.T) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- f.scheduler.Run(ctx) }()
	return func() {
		t.Helper()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run: %v, want nil after cancel", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Run did not return after cancel")
		}
	}
}

// waitForPass waits until a pass lists the users.
func (f *schedulerFixture) waitForPass(t *testing.T) {
	t.Helper()
	select {
	case <-f.users.listed:
	case <-time.After(time.Second):
		t.Fatal("no pass within a second")
	}
}

// fakeUsers returns list, or err. Each List call is reported on listed when
// there is room, without blocking.
type fakeUsers struct {
	list   []store.User
	err    error
	listed chan struct{}
}

func (f *fakeUsers) List(_ context.Context) ([]store.User, error) {
	select {
	case f.listed <- struct{}{}:
	default:
	}
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

// assertLogged checks that one line of logs contains every part.
func assertLogged(t *testing.T, logs *bytes.Buffer, parts ...string) {
	t.Helper()
	if !logged(logs, parts) {
		t.Errorf("no log line contains all of %q; logs:\n%s", parts, logs.String())
	}
}

// assertNotLogged checks that no line of logs contains every part.
func assertNotLogged(t *testing.T, logs *bytes.Buffer, parts ...string) {
	t.Helper()
	if logged(logs, parts) {
		t.Errorf("a log line contains all of %q; logs:\n%s", parts, logs.String())
	}
}

// logged reports whether one line of logs contains every part.
func logged(logs *bytes.Buffer, parts []string) bool {
	for line := range strings.Lines(logs.String()) {
		found := true
		for _, p := range parts {
			if !strings.Contains(line, p) {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}

// The real type satisfies the interface.
var _ Users = (*store.UserStore)(nil)
