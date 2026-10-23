package worker

import (
	"errors"
	"slices"
	"testing"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

func TestProcess_athleteEvents(t *testing.T) {
	testCases := []struct {
		name        string
		updates     string
		deleteErr   error
		wantErr     error
		wantDeleted bool
	}{
		{name: "deauthorization deletes the user", updates: `{"authorized":"false"}`, wantDeleted: true},
		// Positive control: an athlete update that is not a deauthorization.
		{name: "other athlete update is ignored", updates: `{"profile":"changed"}`},
		{name: "authorized true is ignored", updates: `{"authorized":"true"}`},
		{name: "no updates are ignored", updates: `{}`},
		// Retrying cannot fix these: log, done, no deletion.
		{name: "malformed updates are ignored", updates: `{"authorized":`},
		{name: "boolean instead of string is ignored", updates: `{"authorized":false}`},
		{name: "delete error is returned for a retry", updates: `{"authorized":"false"}`, deleteErr: errDB, wantErr: errDB},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, nil)
			f.users.deleteErr = tc.deleteErr

			err := f.worker.process(t.Context(), deauthEvent(tc.updates))

			if tc.wantErr == nil && err != nil {
				t.Fatalf("process: got error %v, wanted nil", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("process: got error %v, wanted one wrapping %v", err, tc.wantErr)
			}
			deleted := f.users.deletedAthletes()
			if tc.wantDeleted {
				// object_id of an athlete event is the athlete's Strava ID.
				testutil.AssertEqual(t, "deleted athletes", true, slices.Equal([]int64{athleteID}, deleted))
			} else if tc.deleteErr == nil && len(deleted) != 0 {
				t.Errorf("deleted athletes: got %v, wanted none", deleted)
			}
			// Athlete events never touch Strava or activities.
			testutil.AssertEqual(t, "Activity calls", 0, f.clients.fetchCount())
			saved, removed := f.activities.calls()
			testutil.AssertEqual(t, "activity changes", 0, len(saved)+len(removed))
		})
	}
}

func TestProcess_deauthorizationOfUnknownAthlete(t *testing.T) {
	// A repeated event, or someone who never was a user: nothing to delete,
	// and not an error.
	f := newFixture(t, nil)
	e := deauthEvent(`{"authorized":"false"}`)
	e.ObjectID, e.OwnerID = 999, 999

	if err := f.worker.process(t.Context(), e); err != nil {
		t.Fatalf("process: %v", err)
	}
	testutil.AssertEqual(t, "delete attempted", true, slices.Equal([]int64{999}, f.users.deletedAthletes()))
}

func deauthEvent(updates string) store.Event {
	return store.Event{ID: 1, ObjectType: "athlete", ObjectID: athleteID, AspectType: "update", OwnerID: athleteID, Updates: updates}
}
