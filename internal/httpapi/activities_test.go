package httpapi

import (
	"encoding/base64"
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
