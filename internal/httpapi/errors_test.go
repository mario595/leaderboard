package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestWriteServerError(t *testing.T) {
	tests := []struct {
		name      string
		cancelled bool // the request's context is cancelled: the client is gone
		err       error
		// What the response and the single log line must look like.
		wantStatus int
		wantLevel  string
		wantMsg    string
	}{
		{
			name:       "client_gone",
			cancelled:  true,
			err:        fmt.Errorf("get user by id: %w", context.Canceled),
			wantStatus: 499,
			wantLevel:  "DEBUG",
			wantMsg:    "request canceled by client",
		},
		{
			// Positive control: a real failure is still an error.
			name:       "store_failure",
			cancelled:  false,
			err:        errors.New("disk I/O error"),
			wantStatus: http.StatusInternalServerError,
			wantLevel:  "ERROR",
			wantMsg:    "look up session user",
		},
		{
			// The error says canceled, but the request is alive: some other
			// context was cancelled, which is a bug on our side.
			name:       "canceled_but_not_the_request",
			cancelled:  false,
			err:        fmt.Errorf("get user by id: %w", context.Canceled),
			wantStatus: http.StatusInternalServerError,
			wantLevel:  "ERROR",
			wantMsg:    "look up session user",
		},
		{
			// A deadline is our own timeout running out, not the client
			// leaving, even if the client is gone too.
			name:       "deadline_exceeded",
			cancelled:  true,
			err:        fmt.Errorf("get user by id: %w", context.DeadlineExceeded),
			wantStatus: http.StatusInternalServerError,
			wantLevel:  "ERROR",
			wantMsg:    "look up session user",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			// Level Debug, or the Debug line is dropped before it reaches buf.
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/me", nil)
			if tc.cancelled {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			writeServerError(rec, req, logger, "look up session user", tc.err, "user_id", int64(42))

			testutil.AssertEqual(t, "status", tc.wantStatus, rec.Code)
			if tc.wantStatus == 499 {
				// No one reads it: no body, nothing to encode.
				testutil.AssertEqual(t, "body", "", rec.Body.String())
			} else {
				testutil.AssertEqual(t, "body", `{"error":"internal error"}`+"\n", rec.Body.String())
			}

			lines := logLines(t, &buf)
			if len(lines) != 1 {
				t.Fatalf("want 1 log line, got %d: %s", len(lines), buf.String())
			}
			line := lines[0]
			testutil.AssertEqual[any](t, "level", tc.wantLevel, line["level"])
			testutil.AssertEqual[any](t, "msg", tc.wantMsg, line["msg"])
			// The caller's attributes and the error are kept on both paths.
			testutil.AssertEqual[any](t, "user_id", float64(42), line["user_id"])
			testutil.AssertEqual[any](t, "error", tc.err.Error(), line["error"])
			if tc.wantStatus == 499 {
				testutil.AssertEqual[any](t, "path", "/api/me", line["path"])
			}
		})
	}
}

// logLines decodes every JSON line slog wrote to buf.
func logLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, raw := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if raw == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatalf("log line %q is not JSON: %v", raw, err)
		}
		lines = append(lines, m)
	}
	return lines
}

// errCanceledQuery is what a store returns when the request's context was
// cancelled mid-query: context.Canceled, wrapped.
var errCanceledQuery = fmt.Errorf("query: %w", context.Canceled)

// clientGoneCase is one row of the handler tests for writeServerError: a
// store call fails with err, on a cancelled request or not.
type clientGoneCase struct {
	name       string
	cancelled  bool
	wantStatus int
	wantLevel  string
	wantMsg    string
	wantAttrs  map[string]any // attributes the single log line must carry
}

// assertServerErrorResponse checks a handler's answer to a failed store call:
// the status, an empty body for 499 or a JSON error otherwise, and exactly
// one log line with the wanted level, message and attributes.
func assertServerErrorResponse(t *testing.T, tc clientGoneCase, rec *httptest.ResponseRecorder, logs *bytes.Buffer) {
	t.Helper()

	testutil.AssertEqual(t, "status", tc.wantStatus, rec.Code)
	if tc.wantStatus == 499 {
		testutil.AssertEqual(t, "body", "", rec.Body.String())
	} else {
		testutil.AssertEqual(t, "body", `{"error":"internal error"}`+"\n", rec.Body.String())
	}

	lines := logLines(t, logs)
	if len(lines) != 1 {
		t.Fatalf("want 1 log line, got %d: %s", len(lines), logs.String())
	}
	testutil.AssertEqual[any](t, "level", tc.wantLevel, lines[0]["level"])
	testutil.AssertEqual[any](t, "msg", tc.wantMsg, lines[0]["msg"])
	for k, v := range tc.wantAttrs {
		testutil.AssertEqual(t, k, v, lines[0][k])
	}
}
