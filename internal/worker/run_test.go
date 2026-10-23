package worker

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
	"github.com/mario595/leaderboard/internal/testutil"
)

var found = fetchResult{a: strava.Activity{ID: activityID, SportType: "Run", MovingTime: time.Hour}}

func TestRun_drainsPendingEventsAtStart(t *testing.T) {
	// Left over from before a restart: no wake will come for them.
	f := newFixture(t, []fetchResult{found, found, found})
	f.worker.pollInterval = time.Hour
	for id := int64(1); id <= 3; id++ {
		f.events.add(eventWithID(id))
	}

	stop := startRun(t, f.worker)
	waitFor(t, "3 events done", func() bool { return len(f.events.doneIDs()) == 3 })
	stop()

	testutil.AssertEqual(t, "processed oldest first", true, slices.Equal([]int64{1, 2, 3}, f.events.doneIDs()))
}

func TestRun_wakeProcessesANewEvent(t *testing.T) {
	f := newFixture(t, []fetchResult{found, found})
	f.worker.pollInterval = time.Hour // only the wake can trigger processing
	wake := make(chan struct{}, 1)
	f.worker.wake = wake

	stop := startRun(t, f.worker)
	defer stop()

	// Two rounds: the worker must go back to waiting after the first one.
	for id := int64(1); id <= 2; id++ {
		f.events.add(eventWithID(id))
		wake <- struct{}{}
		waitFor(t, "event done after a wake", func() bool { return len(f.events.doneIDs()) == int(id) })
	}
}

func TestRun_pollPicksUpDueEvents(t *testing.T) {
	// No wake at all (as for a retry that becomes due): the poll finds it.
	f := newFixture(t, []fetchResult{found})
	f.worker.pollInterval = 20 * time.Millisecond

	stop := startRun(t, f.worker)
	defer stop()
	f.events.add(eventWithID(1))

	waitFor(t, "event done after a poll", func() bool { return len(f.events.doneIDs()) == 1 })
}

func TestRun_queueErrorDoesNotStopTheWorker(t *testing.T) {
	f := newFixture(t, []fetchResult{found})
	f.worker.pollInterval = 20 * time.Millisecond
	f.events.nextErr = errors.New("database is locked")
	f.events.add(eventWithID(1))

	stop := startRun(t, f.worker)
	defer stop()

	waitFor(t, "event done after a queue error", func() bool { return len(f.events.doneIDs()) == 1 })
}

func TestRun_cancelReturnsPromptly(t *testing.T) {
	f := newFixture(t, nil)
	f.worker.pollInterval = time.Hour // waiting: only cancellation can end Run

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- f.worker.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run after cancel: got error %v, wanted nil (shutdown is not an error)", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return within 1s of cancellation")
	}
}

func TestRun_cancelFinishesTheCurrentEvent(t *testing.T) {
	f := newFixture(t, []fetchResult{found, found})
	f.worker.pollInterval = time.Hour
	f.clients.gate = make(chan struct{})
	f.clients.started = make(chan struct{}, 2)
	f.events.add(eventWithID(1))
	f.events.add(eventWithID(2))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- f.worker.Run(ctx) }()

	// Shutdown arrives while event 1 is being fetched.
	<-f.clients.started
	cancel()
	close(f.clients.gate)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	// Event 1 completed normally: its work was not cancelled half-way, so it
	// is done, not retried. Event 2 was not started.
	testutil.AssertEqual(t, "done events", true, slices.Equal([]int64{1}, f.events.doneIDs()))
	testutil.AssertEqual(t, "retried events", 0, f.events.retriedCount())
	f.clients.mu.Lock()
	ctxErr := f.clients.ctxErr
	f.clients.mu.Unlock()
	if ctxErr != nil {
		t.Errorf("the in-flight event's context was cancelled (%v); shutdown must let it finish", ctxErr)
	}
}

func eventWithID(id int64) store.Event {
	e := activityEvent("create")
	e.ID = id
	return e
}

// startRun runs w until the returned stop function is called. stop waits for
// Run to return and fails the test if it takes over a second.
func startRun(t *testing.T, w *Worker) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()
	return func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Run: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("Run did not return within 1s of cancellation")
		}
	}
}

// waitFor polls cond until it is true, or fails the test after 2 seconds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}
