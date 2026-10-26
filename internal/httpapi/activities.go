package httpapi

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mario595/leaderboard/internal/store"
)

// cursorTimeLayout is the time format inside a cursor. It is part of the API's
// cursor format, separate from how the store saves start times, even though
// both look the same today.
const cursorTimeLayout = "2006-01-02T15:04:05"

// errInvalidCursor means a cursor could not be decoded. Clients cannot fix an
// opaque cursor, so it carries no detail: they start again from page one.
var errInvalidCursor = errors.New("invalid cursor")

// encodeCursor turns a position in the activity list into an opaque,
// URL-safe string (base64 without padding of "start|id"). Clients must pass it
// back unchanged and never build or read one.
func encodeCursor(c store.ActivityCursor) string {
	str := fmt.Sprintf("%s|%d", c.StartLocal.Format(cursorTimeLayout), c.StravaID)
	return base64.RawURLEncoding.EncodeToString([]byte(str))
}

// decodeCursor reverses encodeCursor. Any malformed input, including a
// non-positive ID, gives errInvalidCursor.
func decodeCursor(s string) (store.ActivityCursor, error) {
	dec, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return store.ActivityCursor{}, errInvalidCursor
	}

	b, a, found := strings.Cut(string(dec), "|")
	if !found {
		return store.ActivityCursor{}, errInvalidCursor
	}

	startLocal, err := time.Parse(cursorTimeLayout, b)
	if err != nil {
		return store.ActivityCursor{}, errInvalidCursor
	}
	stravaID, err := strconv.ParseInt(a, 10, 64)
	if err != nil {
		return store.ActivityCursor{}, errInvalidCursor
	}
	if stravaID <= 0 {
		return store.ActivityCursor{}, errInvalidCursor
	}
	return store.ActivityCursor{
		StartLocal: startLocal,
		StravaID:   stravaID,
	}, nil
}
