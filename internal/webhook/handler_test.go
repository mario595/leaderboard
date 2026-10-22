package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

const testVerifyToken = "s3cret-verify-token"

func TestVerify(t *testing.T) {
	testCases := []struct {
		name       string
		mode       string
		token      string
		wantStatus int
	}{
		// Positive control: without it, a handler that always answers 403
		// would pass every other row.
		{name: "valid", mode: "subscribe", token: testVerifyToken, wantStatus: http.StatusOK},
		{name: "wrong token", mode: "subscribe", token: "guess", wantStatus: http.StatusForbidden},
		{name: "token prefix", mode: "subscribe", token: testVerifyToken[:5], wantStatus: http.StatusForbidden},
		{name: "missing token", mode: "subscribe", token: "", wantStatus: http.StatusForbidden},
		{name: "wrong mode", mode: "unsubscribe", token: testVerifyToken, wantStatus: http.StatusForbidden},
		{name: "missing mode", mode: "", token: testVerifyToken, wantStatus: http.StatusForbidden},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newTestHandler(t, 120475)
			q := url.Values{}
			q.Set("hub.mode", tc.mode)
			q.Set("hub.verify_token", tc.token)
			q.Set("hub.challenge", "15f7d1a91c1f40f8a748fd134752feb3")
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/webhook/strava?"+q.Encode(), nil)
			rec := httptest.NewRecorder()

			h.Verify(rec, req)

			testutil.AssertEqual(t, "status", tc.wantStatus, rec.Code)
			if tc.wantStatus != http.StatusOK {
				// Never echo the challenge to a caller without the token.
				if strings.Contains(rec.Body.String(), "15f7d1a91c1f40f8a748fd134752feb3") {
					t.Errorf("body %q echoes the challenge on a rejected handshake", rec.Body.String())
				}
				return
			}
			testutil.AssertEqual(t, "Content-Type", "application/json", rec.Header().Get("Content-Type"))
			var body map[string]string
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode body %q: %v", rec.Body.String(), err)
			}
			// Exactly the shape Strava expects: one key, with a dot in its name.
			if len(body) != 1 || body["hub.challenge"] != "15f7d1a91c1f40f8a748fd134752feb3" {
				t.Errorf("body: got %v, wanted {\"hub.challenge\": \"15f7d1a91c1f40f8a748fd134752feb3\"}", body)
			}
		})
	}
}

const testSubscriptionID = 120475

func TestReceive_validEvents(t *testing.T) {
	testCases := []struct {
		name string
		body string
		want store.Event
	}{
		{
			name: "activity update",
			body: `{"aspect_type":"update","event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":120475,"updates":{"title":"Morning Run"}}`,
			want: store.Event{ObjectType: "activity", ObjectID: 12345678901, AspectType: "update", OwnerID: 134815, EventTime: time.Unix(1760521800, 0), Updates: `{"title":"Morning Run"}`},
		},
		{
			// Strava sends "updates":{} on create; store it as is.
			name: "activity create",
			body: `{"aspect_type":"create","event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":120475,"updates":{}}`,
			want: store.Event{ObjectType: "activity", ObjectID: 12345678901, AspectType: "create", OwnerID: 134815, EventTime: time.Unix(1760521800, 0), Updates: `{}`},
		},
		{
			// Without "updates" the column must still hold valid JSON, not "".
			name: "updates missing",
			body: `{"aspect_type":"delete","event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":120475}`,
			want: store.Event{ObjectType: "activity", ObjectID: 12345678901, AspectType: "delete", OwnerID: 134815, EventTime: time.Unix(1760521800, 0), Updates: `{}`},
		},
		{
			// Deauthorization (#15): object_id is the athlete.
			name: "athlete deauthorization",
			body: `{"aspect_type":"update","event_time":1760521800,"object_id":134815,"object_type":"athlete","owner_id":134815,"subscription_id":120475,"updates":{"authorized":"false"}}`,
			want: store.Event{ObjectType: "athlete", ObjectID: 134815, AspectType: "update", OwnerID: 134815, EventTime: time.Unix(1760521800, 0), Updates: `{"authorized":"false"}`},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wake := make(chan struct{}, 1)
			h, events := newTestHandlerWithWake(t, testSubscriptionID, wake)

			rec := postEvent(t, h, tc.body)

			testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
			got := events.all()
			if len(got) != 1 {
				t.Fatalf("inserted events: got %d, wanted 1", len(got))
			}
			testutil.AssertEqual(t, "ObjectType", tc.want.ObjectType, got[0].ObjectType)
			testutil.AssertEqual(t, "ObjectID", tc.want.ObjectID, got[0].ObjectID)
			testutil.AssertEqual(t, "AspectType", tc.want.AspectType, got[0].AspectType)
			testutil.AssertEqual(t, "OwnerID", tc.want.OwnerID, got[0].OwnerID)
			testutil.AssertTime(t, "EventTime", tc.want.EventTime, got[0].EventTime)
			testutil.AssertEqual(t, "Updates", tc.want.Updates, got[0].Updates)

			select {
			case <-wake:
			default:
				t.Error("a stored event did not wake the worker")
			}
		})
	}
}

func TestReceive_rejectedEvents(t *testing.T) {
	const valid = `{"aspect_type":"create","event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":120475,"updates":{}}`

	testCases := []struct {
		name           string
		subscriptionID int64 // configured on the handler
		body           string
		wantStatus     int
	}{
		// Positive control: the same body is accepted with the right config.
		{name: "valid", subscriptionID: testSubscriptionID, body: valid, wantStatus: http.StatusOK},
		{name: "other subscription", subscriptionID: 999, body: valid, wantStatus: http.StatusForbidden},
		{
			// Unset config must not match an event that also omits the ID.
			name:           "subscription not configured",
			subscriptionID: 0,
			body:           `{"aspect_type":"create","event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":0}`,
			wantStatus:     http.StatusForbidden,
		},
		{name: "unknown aspect type", subscriptionID: testSubscriptionID, body: strings.Replace(valid, `"create"`, `"archive"`, 1), wantStatus: http.StatusBadRequest},
		{name: "unknown object type", subscriptionID: testSubscriptionID, body: strings.Replace(valid, `"activity"`, `"route"`, 1), wantStatus: http.StatusBadRequest},
		{name: "zero object id", subscriptionID: testSubscriptionID, body: strings.Replace(valid, `"object_id":12345678901`, `"object_id":0`, 1), wantStatus: http.StatusBadRequest},
		{name: "zero owner id", subscriptionID: testSubscriptionID, body: strings.Replace(valid, `"owner_id":134815`, `"owner_id":0`, 1), wantStatus: http.StatusBadRequest},
		{name: "zero event time", subscriptionID: testSubscriptionID, body: strings.Replace(valid, `"event_time":1760521800`, `"event_time":0`, 1), wantStatus: http.StatusBadRequest},
		{name: "malformed JSON", subscriptionID: testSubscriptionID, body: `{"aspect_type":`, wantStatus: http.StatusBadRequest},
		{name: "wrong JSON type", subscriptionID: testSubscriptionID, body: strings.Replace(valid, `"object_id":12345678901`, `"object_id":"12345678901"`, 1), wantStatus: http.StatusBadRequest},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h, events := newTestHandler(t, tc.subscriptionID)

			rec := postEvent(t, h, tc.body)

			testutil.AssertEqual(t, "status", tc.wantStatus, rec.Code)
			wantInserted := 0
			if tc.wantStatus == http.StatusOK {
				wantInserted = 1
			}
			testutil.AssertEqual(t, "inserted events", wantInserted, len(events.all()))
		})
	}
}

func TestReceive_bodyTooLarge(t *testing.T) {
	h, events := newTestHandler(t, testSubscriptionID)
	// A valid event padded past the limit: only the size can reject it.
	padding := strings.Repeat(" ", 8<<10)
	body := `{"aspect_type":"create",` + padding + `"event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":120475}`

	rec := postEvent(t, h, body)

	if rec.Code != http.StatusBadRequest && rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status: got %d, wanted 400 or 413", rec.Code)
	}
	testutil.AssertEqual(t, "inserted events", 0, len(events.all()))
}

func TestReceive_duplicateIsAccepted(t *testing.T) {
	wake := make(chan struct{}, 1)
	h, events := newTestHandlerWithWake(t, testSubscriptionID, wake)
	events.inserted = false // the store already has this event

	rec := postEvent(t, h, `{"aspect_type":"create","event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":120475}`)

	// Strava retried a delivery we already stored: 200, or it keeps retrying.
	testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
	// Nothing new to process: the worker must not be woken.
	select {
	case <-wake:
		t.Error("a duplicate event woke the worker")
	default:
	}
}

func TestReceive_storeErrorIs500(t *testing.T) {
	h, events := newTestHandler(t, testSubscriptionID)
	events.err = errors.New("database is locked")

	rec := postEvent(t, h, `{"aspect_type":"create","event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":120475}`)

	// Not stored, so not acknowledged: a 5xx makes Strava retry.
	testutil.AssertEqual(t, "status", http.StatusInternalServerError, rec.Code)
	if strings.Contains(rec.Body.String(), "database is locked") {
		t.Errorf("body %q leaks the internal error", rec.Body.String())
	}
}

func TestReceive_neverBlocksOnWake(t *testing.T) {
	testCases := []struct {
		name string
		wake chan struct{}
	}{
		{name: "no worker (nil channel)", wake: nil},
		{name: "wake already pending (full channel)", wake: make(chan struct{}, 1)},
		{name: "unbuffered channel nobody reads", wake: make(chan struct{})},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if cap(tc.wake) > 0 {
				tc.wake <- struct{}{}
			}
			h, _ := newTestHandlerWithWake(t, testSubscriptionID, tc.wake)

			done := make(chan int)
			go func() {
				rec := postEvent(t, h, `{"aspect_type":"create","event_time":1760521800,"object_id":12345678901,"object_type":"activity","owner_id":134815,"subscription_id":120475}`)
				done <- rec.Code
			}()

			select {
			case code := <-done:
				testutil.AssertEqual(t, "status", http.StatusOK, code)
			case <-time.After(time.Second):
				t.Fatal("Receive blocked on the wake channel")
			}
		})
	}
}

// postEvent sends body to h.Receive and returns the recorded response.
func postEvent(t *testing.T, h *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/webhook/strava", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Receive(rec, req)
	return rec
}

// newTestHandler returns a Handler with a fake event store and a logger
// writing to a buffer. The wake channel is nil, as it is until #14.
func newTestHandler(t *testing.T, subscriptionID int64) (*Handler, *fakeEvents) {
	t.Helper()
	return newTestHandlerWithWake(t, subscriptionID, nil)
}

func newTestHandlerWithWake(t *testing.T, subscriptionID int64, wake chan struct{}) (*Handler, *fakeEvents) {
	t.Helper()
	events := &fakeEvents{inserted: true}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	return New(events, testVerifyToken, subscriptionID, wake, logger), events
}

// fakeEvents records inserted events. inserted and err are what Insert
// returns.
type fakeEvents struct {
	mu       sync.Mutex
	inserted bool
	err      error
	events   []store.Event
}

func (f *fakeEvents) all() []store.Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.events)
}

func (f *fakeEvents) Insert(_ context.Context, e store.Event) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return false, f.err
	}
	f.events = append(f.events, e)
	return f.inserted, nil
}
