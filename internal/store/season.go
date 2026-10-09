package store

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Season struct {
	ID        int64
	Name      string
	StartDate time.Time
	EndDate   time.Time
	CreatedAt time.Time
}

// Error for invalid seasons
var ErrInvalidSeason = errors.New("invalid season")

// Validates a season. A valid season has a non-empty name and end date after start date.
func (s Season) Validate() error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("%w: Name is required", ErrInvalidSeason)
	}

	if !truncateToDate(s.EndDate).After(truncateToDate(s.StartDate)) {
		return fmt.Errorf("%w: EndDate must be after StartDate", ErrInvalidSeason)
	}

	return nil
}

// ParseDate parses a date in the format "YYYY-MM-DD"
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("ParseDate: %w", err)
	}

	return t, nil
}

// Formats a the date from the time as YYYY-MM-DD
func FormatDate(t time.Time) string {
	return t.Format(time.DateOnly)
}

func truncateToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
