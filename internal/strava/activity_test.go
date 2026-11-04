package strava

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestActivityDTO_toActivity(t *testing.T) {
	testCases := []struct {
		fixture string
		want    Activity
	}{
		{
			// workout_type and description are null: they must map to 0 and "".
			// start_date (UTC) is already the 16th; the day that counts is the
			// local 15th, so StartLocal must come from start_date_local.
			fixture: "activity_run.json",
			want: Activity{
				ID:          12345678901,
				SportType:   "Run",
				StartLocal:  time.Date(2026, 10, 15, 21, 30, 0, 0, time.UTC),
				MovingTime:  2735 * time.Second,
				Distance:    8123.4,
				WorkoutType: 0,
				Description: "",
			},
		},
		{
			// sport_type, not the legacy type ("Ride"), drives scoring.
			fixture: "activity_race_ride.json",
			want: Activity{
				ID:          12345678902,
				SportType:   "GravelRide",
				StartLocal:  time.Date(2026, 10, 17, 9, 0, 0, 0, time.UTC),
				MovingTime:  8102 * time.Second,
				Distance:    61234.0,
				WorkoutType: 11,
				Description: "Hard day out. #competition",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.fixture, func(t *testing.T) {
			dto := decodeFixture(t, tc.fixture)

			got, err := dto.toActivity()
			if err != nil {
				t.Fatalf("toActivity: %v", err)
			}

			if !got.StartLocal.Equal(tc.want.StartLocal) || got.StartLocal.Location() != time.UTC {
				t.Errorf("StartLocal: got %v, wanted %v in UTC", got.StartLocal, tc.want.StartLocal)
			}
			// StartLocal is checked above with Equal; compare the rest with ==.
			got.StartLocal, tc.want.StartLocal = time.Time{}, time.Time{}
			if got != tc.want {
				t.Errorf("toActivity:\n got  %+v\n want %+v", got, tc.want)
			}
		})
	}
}

func TestClient_activity(t *testing.T) {
	fake := newFakeStrava(t)
	client := newTestAPI(fake).ForUser(7, validToken())

	got, err := client.Activity(t.Context(), 12345678902)
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}

	// The ID is above 2^32: formatting it as anything but a full int64 breaks the path.
	testutil.AssertEqual(t, "request path", "/activities/12345678902", fake.lastAPIPath())
	testutil.AssertEqual(t, "Authorization", "Bearer old-access", fake.lastAPIAuth())
	// Mapping details are covered by TestActivityDTO_toActivity; check it ran.
	testutil.AssertEqual(t, "ID", int64(12345678902), got.ID)
	testutil.AssertEqual(t, "SportType", "GravelRide", got.SportType)
	testutil.AssertEqual(t, "MovingTime", 8102*time.Second, got.MovingTime)
}

func TestClient_listActivities(t *testing.T) {
	fake := newFakeStrava(t)
	client := newTestAPI(fake).ForUser(7, validToken())
	after := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, 12, 2, 0, 0, 0, 0, time.UTC)

	got, err := client.ListActivities(t.Context(), after, before, 1, 200)
	if err != nil {
		t.Fatalf("ListActivities: %v", err)
	}

	testutil.AssertEqual(t, "request path", "/athlete/activities", fake.lastAPIPath())
	testutil.AssertEqual(t, "Authorization", "Bearer old-access", fake.lastAPIAuth())
	q := fake.lastListQuery()
	testutil.AssertEqual(t, "after", "1788220800", q.Get("after"))   // 2026-09-01T00:00:00Z
	testutil.AssertEqual(t, "before", "1796169600", q.Get("before")) // 2026-12-02T00:00:00Z
	testutil.AssertEqual(t, "page", "1", q.Get("page"))
	testutil.AssertEqual(t, "per_page", "200", q.Get("per_page"))

	if len(got) != 2 {
		t.Fatalf("ListActivities returned %d activities, want 2", len(got))
	}
	// Same mapping as Activity: the local wall clock, workout_type null -> 0.
	run := got[0]
	testutil.AssertEqual(t, "run ID", int64(12345678901), run.ID)
	testutil.AssertEqual(t, "run SportType", "Run", run.SportType)
	testutil.AssertTime(t, "run StartLocal", time.Date(2026, 10, 15, 21, 30, 0, 0, time.UTC), run.StartLocal)
	testutil.AssertEqual(t, "run MovingTime", 2735*time.Second, run.MovingTime)
	testutil.AssertEqual(t, "run Distance", 8123.4, run.Distance)
	testutil.AssertEqual(t, "run WorkoutType", 0, run.WorkoutType)
	// A summary has no description.
	testutil.AssertEqual(t, "run Description", "", run.Description)

	ride := got[1]
	testutil.AssertEqual(t, "ride ID", int64(12345678902), ride.ID)
	testutil.AssertEqual(t, "ride SportType", "GravelRide", ride.SportType)
	testutil.AssertEqual(t, "ride WorkoutType", 11, ride.WorkoutType)
}

func TestClient_listActivitiesOtherPage(t *testing.T) {
	fake := newFakeStrava(t)
	client := newTestAPI(fake).ForUser(7, validToken())

	got, err := client.ListActivities(t.Context(), time.Unix(0, 0), time.Now(), 3, 50)
	if err != nil {
		t.Fatalf("ListActivities: %v", err)
	}

	testutil.AssertEqual(t, "page", "3", fake.lastListQuery().Get("page"))
	testutil.AssertEqual(t, "per_page", "50", fake.lastListQuery().Get("per_page"))
	// Past the last page, Strava answers []: no activities, no error.
	testutil.AssertEqual(t, "activities", 0, len(got))
}

func TestClient_listActivitiesErrors(t *testing.T) {
	t.Run("rate limited", func(t *testing.T) {
		fake := newFakeStrava(t)
		fake.apiStatus = http.StatusTooManyRequests
		client := newTestAPI(fake).ForUser(7, validToken())

		_, err := client.ListActivities(t.Context(), time.Unix(0, 0), time.Now(), 1, 200)

		var rl *RateLimitError
		if !errors.As(err, &rl) {
			t.Errorf("ListActivities with status 429: error %v, want a *RateLimitError", err)
		}
	})
	t.Run("bad start date in one activity", func(t *testing.T) {
		fake := newFakeStrava(t)
		fake.listBody = `[{"id":1,"sport_type":"Run","start_date_local":"2026-10-15T21:30:00Z"},
			{"id":2,"sport_type":"Run","start_date_local":"15/10/2026"}]`
		client := newTestAPI(fake).ForUser(7, validToken())

		got, err := client.ListActivities(t.Context(), time.Unix(0, 0), time.Now(), 1, 200)

		// Half a page would look like the last page to a caller: fail it all.
		if err == nil {
			t.Fatalf("ListActivities with an unparseable date: got %d activities and no error", len(got))
		}
	})
}

func TestActivityDTO_toActivityBadDate(t *testing.T) {
	dto := decodeFixture(t, "activity_run.json")
	dto.StartDateLocal = "15/10/2026 21:30"

	if _, err := dto.toActivity(); err == nil {
		t.Error("toActivity with an unparseable start_date_local: got nil error")
	}
}

// decodeFixture decodes a recorded Strava response from testdata.
func decodeFixture(t *testing.T, name string) activityDTO {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var dto activityDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	return dto
}
