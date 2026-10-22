package strava

import (
	"encoding/json"
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
