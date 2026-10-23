package store

import (
	"context"
	"fmt"
	"time"
)

// Standing is one member's total for a date range.
type Standing struct {
	UserID    int64
	FirstName string
	LastName  string
	Points    int
}

// standingsQuery sums each user's points between two days (YYYY-MM-DD, both
// inclusive). The date filter is in the LEFT JOIN's ON clause: in WHERE it
// would drop users without activities in the range.
const standingsQuery = `
	SELECT u.id, u.first_name, u.last_name, COALESCE(SUM(a.points), 0) AS points
	FROM users u
	LEFT JOIN activities a
		ON a.user_id = u.id AND a.day BETWEEN ? AND ?
	GROUP BY u.id
	ORDER BY points DESC, u.first_name, u.last_name, u.id;
`

// Standings returns every user's total points for activities whose local day
// lies between from and to, both inclusive (only the calendar dates count).
// Users without points in the range are included with 0. The result is
// ordered by points, highest first, then by first name, last name and user
// ID; it is an empty, non-nil slice when there are no users.
func (s *ActivityStore) Standings(ctx context.Context, from, to time.Time) ([]Standing, error) {
	result := []Standing{}
	rows, err := s.db.QueryContext(ctx, standingsQuery, FormatDate(from), FormatDate(to))
	if err != nil {
		return nil, fmt.Errorf("standings from %s to %s: %w", FormatDate(from), FormatDate(to), err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		standing, err := scanStanding(rows)
		if err != nil {
			return nil, fmt.Errorf("standings from %s to %s: %w", FormatDate(from), FormatDate(to), err)
		}
		result = append(result, standing)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("standings from %s to %s: %w", FormatDate(from), FormatDate(to), err)
	}

	return result, nil
}

// scanStanding scans one row of standingsQuery.
func scanStanding(r rowScanner) (Standing, error) {
	var s Standing
	if err := r.Scan(&s.UserID, &s.FirstName, &s.LastName, &s.Points); err != nil {
		return Standing{}, fmt.Errorf("scan standing: %w", err)
	}
	return s, nil
}
