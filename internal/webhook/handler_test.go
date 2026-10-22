package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

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

// newTestHandler returns a Handler with a fake event store and a logger
// writing to a buffer. The wake channel is nil, as it is until #14.
func newTestHandler(t *testing.T, subscriptionID int64) (*Handler, *fakeEvents) {
	t.Helper()
	events := &fakeEvents{inserted: true}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	return New(events, testVerifyToken, subscriptionID, nil, logger), events
}

// fakeEvents records inserted events. inserted and err are what Insert
// returns.
type fakeEvents struct {
	mu       sync.Mutex
	inserted bool
	err      error
	events   []store.Event
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
