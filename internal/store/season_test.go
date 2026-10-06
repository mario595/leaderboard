package store

import (
	"errors"
	"testing"
	"time"
)

func TestSeasonValidation(t *testing.T) {
	testCases := []struct {
		name   string
		season Season
		valid  bool
	}{
		{
			name:  "Valid season",
			valid: true,
			season: Season{
				Name:      "valid season",
				StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:  "Empty name season",
			valid: false,
			season: Season{
				Name:      "",
				StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:  "Space only name season",
			valid: false,
			season: Season{
				Name:      "     ",
				StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:  "Start date after End date",
			valid: false,
			season: Season{
				Name:      "Season",
				StartDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:  "Start date same day as End date",
			valid: false,
			season: Season{
				Name:      "Season",
				StartDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
			},
		},
		{
			name:  "Time is ignored",
			valid: false,
			season: Season{
				Name:      "Season",
				StartDate: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
				EndDate:   time.Date(2026, 1, 5, 1, 0, 0, 0, time.UTC),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.season.Validate()
			if tc.valid && err != nil {
				t.Errorf("Unexpected error %v", err)
			}
			if !tc.valid && !errors.Is(err, ErrInvalidSeason) {
				t.Errorf("Unexpected error. Wanted ErrInvalidSeason, got %v", err)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	date, err := ParseDate("2026-11-20")
	if err != nil {
		t.Errorf("ParseDate: %v", err)
	}
	expected := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	if date != expected {
		t.Errorf("Incorrect date parsed, expected: %v, got %v ", expected, date)
	}

	_, err = ParseDate("2026/11/20")
	if err == nil {
		t.Errorf("Expected error")
	}
}

func TestFormatDate(t *testing.T) {
	date := FormatDate(time.Date(2026, 11, 20, 10, 10, 10, 10, time.UTC))
	if date != "2026-11-20" {
		t.Errorf("Unexpected date formatted. Expected 2026-11-20, got %v", date)
	}
}
