package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestWriteJSON(t *testing.T) {

	testCases := []struct {
		name          string
		status        int
		body          any
		expectedKey   string
		expectedValue string
	}{
		{
			name:          "200",
			status:        http.StatusOK,
			body:          map[string]string{"someKey": "someValue"},
			expectedKey:   "someKey",
			expectedValue: "someValue",
		},
		{
			name:          "400",
			status:        http.StatusBadRequest,
			body:          map[string]string{"k": "v"},
			expectedKey:   "k",
			expectedValue: "v",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			logger := slog.New(slog.DiscardHandler)

			writeJSON(rec, logger, tc.status, tc.body)

			testutil.AssertEqual(t, "status", tc.status, rec.Result().StatusCode)
			testutil.AssertEqual(t, "Content-Type", "application/json", rec.Result().Header.Get("Content-Type"))
			var got map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decoding: %v", err)
			}
			testutil.AssertEqual(t, "body", tc.expectedValue, got[tc.expectedKey])
		})
	}
}

func TestWriteError(t *testing.T) {

	testCases := []struct {
		name    string
		status  int
		message string
	}{
		{
			name:    "200",
			status:  http.StatusOK,
			message: "some error",
		},
		{
			name:    "400",
			status:  http.StatusBadRequest,
			message: "another error",
		},
		{
			name:    "500",
			status:  http.StatusInternalServerError,
			message: "internal server error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			logger := slog.New(slog.DiscardHandler)

			writeError(rec, logger, tc.status, tc.message)

			testutil.AssertEqual(t, "status", tc.status, rec.Result().StatusCode)
			testutil.AssertEqual(t, "Content-Type", "application/json", rec.Result().Header.Get("Content-Type"))
			var got map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
				t.Fatalf("decoding: %v", err)
			}

			testutil.AssertEqual(t, "body", tc.message, got["error"])
		})
	}
}

// TestWriteJSON_encodeFailure checks what happens when the value cannot be
// encoded. By then the status line has already been sent, so the response keeps
// the requested status, and the only thing left to do is to log the failure.
func TestWriteJSON_encodeFailure(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	rec := httptest.NewRecorder()

	// A channel has no JSON representation, so encoding it fails.
	writeJSON(rec, logger, http.StatusOK, make(chan int))

	testutil.AssertEqual(t, "status", http.StatusOK, rec.Code)
	if !strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("logs = %q, want an ERROR line about the failed encoding", logs.String())
	}
}
