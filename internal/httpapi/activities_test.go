package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

func TestCursor_roundTrip(t *testing.T) {
	testCases := []store.ActivityCursor{
		{StartLocal: time.Date(2026, 10, 15, 7, 30, 0, 0, time.UTC), StravaID: 12345678901},
		{StartLocal: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), StravaID: 1},
		{StartLocal: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), StravaID: 9223372036854775807},
	}
	for _, c := range testCases {
		encoded := encodeCursor(c)

		// Usable as-is in a query string: URL-safe alphabet, no padding.
		if strings.ContainsAny(encoded, "+/=") {
			t.Errorf("encodeCursor(%+v) = %q: needs escaping in a URL", c, encoded)
		}

		got, err := decodeCursor(encoded)
		if err != nil {
			t.Fatalf("decodeCursor(encodeCursor(%+v)): %v", c, err)
		}
		testutil.AssertTime(t, "StartLocal", c.StartLocal, got.StartLocal)
		testutil.AssertEqual(t, "StravaID", c.StravaID, got.StravaID)
	}
}

func TestCursor_opaque(t *testing.T) {
	// Clients must not read the cursor: it is not the plain text.
	c := store.ActivityCursor{StartLocal: time.Date(2026, 10, 15, 7, 30, 0, 0, time.UTC), StravaID: 42}
	if encoded := encodeCursor(c); strings.Contains(encoded, "2026") {
		t.Errorf("encodeCursor(%+v) = %q: the cursor shows its content", c, encoded)
	}
}

func TestDecodeCursor_malformed(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{name: "not base64", input: "!!!"},
		{name: "standard base64 with padding", input: "MjAyNi0xMC0xNVQwNzozMDowMHw0Mg=="},
		{name: "no separator", input: b64("2026-10-15T07:30:00")},
		{name: "bad time", input: b64("15/10/2026|42")},
		{name: "time without seconds", input: b64("2026-10-15T07:30|42")},
		{name: "ID not a number", input: b64("2026-10-15T07:30:00|abc")},
		{name: "zero ID", input: b64("2026-10-15T07:30:00|0")},
		{name: "negative ID", input: b64("2026-10-15T07:30:00|-1")},
		{name: "empty", input: ""},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := decodeCursor(tc.input); err == nil {
				t.Errorf("decodeCursor(%q) = %+v, wanted an error", tc.input, got)
			}
		})
	}

	// Positive control: the same helper builds a valid cursor.
	if _, err := decodeCursor(b64("2026-10-15T07:30:00|42")); err != nil {
		t.Errorf("decodeCursor of a valid cursor: %v", err)
	}
}

// b64 encodes s the way cursors are encoded, to build malformed ones.
func b64(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func TestActivitiesHandler_firstPage(t *testing.T) {
	f := newActivitiesFixture(t)

	rec := f.get(t, "/api/me/activities")

	testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
	// The session's user, the current season's range, no cursor, and one row
	// more than the default page size (50) to detect a next page.
	testutil.AssertEqual(t, "user", int64(7), f.lister.userID)
	testutil.AssertEqual(t, "from", "2026-09-01", store.FormatDate(f.lister.from))
	testutil.AssertEqual(t, "to", "2026-12-31", store.FormatDate(f.lister.to))
	testutil.AssertEqual(t, "after", true, f.lister.after == nil)
	testutil.AssertEqual(t, "limit asked", 51, f.lister.limit)
	assertJSON(t, rec.Body.String(), `{
		"season": {"id": 3, "name": "Autumn 2026", "startDate": "2026-09-01", "endDate": "2026-12-31"},
		"activities": [
			{"id": 104, "date": "2026-10-17", "startLocal": "2026-10-17T09:00:00", "sportType": "GravelRide",
			 "movingTime": 8102, "distance": 61234, "competition": true, "points": 4, "zeroReason": ""},
			{"id": 103, "date": "2026-10-16", "startLocal": "2026-10-16T18:00:00", "sportType": "Run",
			 "movingTime": 2700, "distance": 9000, "competition": true, "points": 4, "zeroReason": ""},
			{"id": 102, "date": "2026-10-16", "startLocal": "2026-10-16T07:00:00", "sportType": "Walk",
			 "movingTime": 3600, "distance": 5000, "competition": false, "points": 0, "zeroReason": "sport"},
			{"id": 101, "date": "2026-10-15", "startLocal": "2026-10-15T21:30:00", "sportType": "Run",
			 "movingTime": 1200, "distance": 3000, "competition": false, "points": 0, "zeroReason": "short"}
		],
		"nextCursor": null
	}`)
	// The description is stored only for the #competition check: never sent.
	if strings.Contains(rec.Body.String(), "description") || strings.Contains(rec.Body.String(), "#competition") {
		t.Errorf("body %s exposes the description", rec.Body.String())
	}
}

func TestZeroReason(t *testing.T) {
	activity := func(sport string, moving time.Duration, points int) store.Activity {
		return store.Activity{SportType: sport, MovingTime: moving, Points: points}
	}
	testCases := []struct {
		name string
		a    store.Activity
		want string
	}{
		// Positive control: an activity that scored needs no reason.
		{name: "scored", a: activity("Run", 45*time.Minute, 2), want: ""},
		{name: "competition scored", a: activity("Swim", 45*time.Minute, 4), want: ""},
		{name: "sport that does not count", a: activity("Walk", time.Hour, 0), want: "sport"},
		{name: "e-bike", a: activity("EBikeRide", time.Hour, 0), want: "sport"},
		// Two reasons apply; the sport explains it (a longer e-bike ride would
		// not score either).
		{name: "short e-bike ride", a: activity("EBikeRide", 10*time.Minute, 0), want: "sport"},
		{name: "too short", a: activity("Run", 20*time.Minute, 0), want: "short"},
		// The boundary is the scoring rule's: 30 minutes counts (#9).
		{name: "29:59 is short", a: activity("Run", 29*time.Minute+59*time.Second, 0), want: "short"},
		// Eligible but 0 points: two better activities that day took the slots.
		{name: "exactly 30 minutes, capped", a: activity("Run", 30*time.Minute, 0), want: "daily_cap"},
		{name: "eligible ride, capped", a: activity("GravelRide", 2*time.Hour, 0), want: "daily_cap"},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			testutil.AssertEqual(t, "zeroReason", tc.want, zeroReason(tc.a))
		})
	}
}

func TestActivitiesHandler_pages(t *testing.T) {
	f := newActivitiesFixture(t)

	// limit=2: the store gets asked for 3; it has more, so there is a next page.
	rec := f.get(t, "/api/me/activities?limit=2")

	testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
	testutil.AssertEqual(t, "limit asked", 3, f.lister.limit)
	var page struct {
		Activities []struct {
			ID int64 `json:"id"`
		} `json:"activities"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	testutil.AssertEqual(t, "rows on the page", 2, len(page.Activities))
	if page.NextCursor == nil {
		t.Fatal("nextCursor is null, wanted a cursor")
	}
	// The cursor points at the last row of this page (103), not at the extra
	// row used to detect the next page (102).
	c, err := decodeCursor(*page.NextCursor)
	if err != nil {
		t.Fatalf("decode nextCursor: %v", err)
	}
	testutil.AssertEqual(t, "cursor ID", int64(103), c.StravaID)
	testutil.AssertEqual(t, "cursor start", "2026-10-16T18:00:00", c.StartLocal.Format(cursorTimeLayout))

	// The next request passes the cursor back: it reaches the store decoded.
	f.get(t, "/api/me/activities?limit=2&cursor="+*page.NextCursor)
	if f.lister.after == nil || f.lister.after.StravaID != 103 {
		t.Errorf("after: got %+v, wanted the cursor of 103", f.lister.after)
	}
}

func TestActivitiesHandler_lastPageExactlyFull(t *testing.T) {
	// The store returns exactly limit rows: no extra row, so no next page.
	f := newActivitiesFixture(t)

	rec := f.get(t, "/api/me/activities?limit=4")

	if !strings.Contains(rec.Body.String(), `"nextCursor":null`) {
		t.Errorf("body %s: wanted \"nextCursor\":null", rec.Body.String())
	}
}

func TestActivitiesHandler_empty(t *testing.T) {
	f := newActivitiesFixture(t)
	f.lister.rows = []store.Activity{}

	rec := f.get(t, "/api/me/activities")

	if !strings.Contains(rec.Body.String(), `"activities":[]`) || !strings.Contains(rec.Body.String(), `"nextCursor":null`) {
		t.Errorf("body %s: wanted an empty list and no next page", rec.Body.String())
	}
}

func TestActivitiesHandler_errors(t *testing.T) {
	testCases := []struct {
		name       string
		path       string
		setup      func(f *activitiesFixture)
		wantStatus int
		wantList   bool // whether the activity store may be called
	}{
		{name: "limit zero", path: "/api/me/activities?limit=0", wantStatus: http.StatusBadRequest},
		{name: "limit above 100", path: "/api/me/activities?limit=101", wantStatus: http.StatusBadRequest},
		{name: "limit negative", path: "/api/me/activities?limit=-1", wantStatus: http.StatusBadRequest},
		{name: "limit not a number", path: "/api/me/activities?limit=ten", wantStatus: http.StatusBadRequest},
		{name: "bad cursor", path: "/api/me/activities?cursor=nonsense", wantStatus: http.StatusBadRequest},
		{name: "bad season", path: "/api/me/activities?season=abc", wantStatus: http.StatusBadRequest},
		{name: "unknown season", path: "/api/me/activities?season=99", wantStatus: http.StatusNotFound},
		{name: "no current season", path: "/api/me/activities", setup: func(f *activitiesFixture) { f.seasons.currentErr = store.ErrNotFound }, wantStatus: http.StatusNotFound},
		{name: "season store fails", path: "/api/me/activities", setup: func(f *activitiesFixture) { f.seasons.currentErr = errStoreDown }, wantStatus: http.StatusInternalServerError},
		{name: "activity store fails", path: "/api/me/activities", setup: func(f *activitiesFixture) { f.lister.err = errStoreDown }, wantStatus: http.StatusInternalServerError, wantList: true},
		// Only reachable through a wiring bug (route not behind requireAuth).
		{name: "no user in context", path: "/api/me/activities", setup: func(f *activitiesFixture) { f.noUser = true }, wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newActivitiesFixture(t)
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
			testutil.AssertEqual(t, "activity store called", tc.wantList, f.lister.called)
		})
	}
}

func TestActivitiesHandler_limitBounds(t *testing.T) {
	// Positive controls for the limit check: 1 and 100 are allowed.
	for _, limit := range []string{"1", "100"} {
		f := newActivitiesFixture(t)
		rec := f.get(t, "/api/me/activities?limit="+limit)
		testutil.AssertEqual(t, "status for limit="+limit, http.StatusOK, rec.Code)
	}
}

type activitiesFixture struct {
	seasons *fakeSeasons
	lister  *fakeLister
	noUser  bool
}

func newActivitiesFixture(t *testing.T) *activitiesFixture {
	t.Helper()
	at := func(s string) time.Time {
		v, err := time.Parse(cursorTimeLayout, s)
		if err != nil {
			t.Fatalf("parse %q: %v", s, err)
		}
		return v
	}
	autumn := store.Season{ID: 3, Name: "Autumn 2026", StartDate: day(t, "2026-09-01"), EndDate: day(t, "2026-12-31")}
	return &activitiesFixture{
		seasons: &fakeSeasons{current: autumn, byID: map[int64]store.Season{3: autumn}},
		lister: &fakeLister{rows: []store.Activity{
			// A race by workout type, scored.
			{StravaID: 104, UserID: 7, SportType: "GravelRide", StartLocal: at("2026-10-17T09:00:00"), MovingTime: 8102 * time.Second, Distance: 61234, WorkoutType: 11, Points: 4},
			// A competition by tag.
			{StravaID: 103, UserID: 7, SportType: "Run", StartLocal: at("2026-10-16T18:00:00"), MovingTime: 45 * time.Minute, Distance: 9000, Description: "Club race #competition", Points: 4},
			// The tag on a sport that does not count: not a competition.
			{StravaID: 102, UserID: 7, SportType: "Walk", StartLocal: at("2026-10-16T07:00:00"), MovingTime: time.Hour, Distance: 5000, Description: "#competition", Points: 0},
			// Too short to score: listed anyway, with 0 points.
			{StravaID: 101, UserID: 7, SportType: "Run", StartLocal: at("2026-10-15T21:30:00"), MovingTime: 20 * time.Minute, Distance: 3000, Points: 0},
		}},
	}
}

func (f *activitiesFixture) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	h := activitiesHandler(f.seasons, f.lister, func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) }, slog.New(slog.DiscardHandler))
	ctx := t.Context()
	if !f.noUser {
		// What requireAuth puts in the context for the logged-in user.
		ctx = context.WithValue(ctx, userKey{}, store.User{ID: 7, FirstName: "Ada"})
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// fakeLister returns at most limit of its rows, in order, ignoring the cursor
// (the store's paging is tested in the store package).
type fakeLister struct {
	rows     []store.Activity
	err      error
	called   bool
	userID   int64
	from, to time.Time
	after    *store.ActivityCursor
	limit    int
}

func (f *fakeLister) ListForUser(_ context.Context, userID int64, from, to time.Time, after *store.ActivityCursor, limit int) ([]store.Activity, error) {
	f.called = true
	f.userID, f.from, f.to, f.after, f.limit = userID, from, to, after, limit
	if f.err != nil {
		return nil, f.err
	}
	return f.rows[:min(limit, len(f.rows))], nil
}
