package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// OverlapError reports that a season overlaps with an existing one.
// Use errors.As to retrieve it and inspect the conflicting season.
type OverlapError struct {
	With Season
}

// Error implements the error interface.
func (e *OverlapError) Error() string {
	return fmt.Sprintf("season overlaps with %q (%s to %s)", e.With.Name, FormatDate(e.With.StartDate), FormatDate(e.With.EndDate))
}

// SeasonStore persists seasons in SQLite.
type SeasonStore struct {
	db  *sql.DB
	now func() time.Time
}

// NewSeasonStore returns a SeasonStore backed by db.
func NewSeasonStore(db *sql.DB) *SeasonStore {
	return &SeasonStore{
		db:  db,
		now: time.Now,
	}
}

// selectSeasonOverlappingQuery finds one season that overlaps a date range.
// Its arguments are the range's end date, then its start date, as YYYY-MM-DD
// (both inclusive). A single date is the range date..date.
const selectSeasonOverlappingQuery = `
		SELECT id, name, start_date, end_date, created_at
		FROM seasons
		WHERE start_date <= ? AND end_date >= ?
		LIMIT 1;
	`

// selectSeasonByIDQuery finds a season by ID.
const selectSeasonByIDQuery = `
		SELECT id, name, start_date, end_date, created_at
		FROM seasons
		WHERE id = ?;
	`

// insertSeasonQuery inserts a season and returns the stored row.
const insertSeasonQuery = `
		INSERT INTO seasons(name, start_date, end_date, created_at)
		VALUES (?, ?, ?, ?)
		RETURNING id, name, start_date, end_date, created_at;	
	`

// listSeasonsQuery lists all seasons.
const listSeasonsQuery = `
		SELECT id, name, start_date, end_date, created_at
		FROM seasons
		ORDER BY start_date;
	`

// Create validates and stores a new season. The overlap check and the insert
// run in one transaction. It returns an error wrapping ErrInvalidSeason if the
// season is invalid, and an *OverlapError if it overlaps an existing season.
func (st *SeasonStore) Create(ctx context.Context, s Season) (Season, error) {
	if err := s.Validate(); err != nil {
		return Season{}, fmt.Errorf("validate: %w", err)
	}
	tx, err := st.db.BeginTx(ctx, nil)
	if err != nil {
		return Season{}, fmt.Errorf("beginTx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	conflictSeason, err := scanSeason(tx.QueryRowContext(ctx,
		selectSeasonOverlappingQuery,
		FormatDate(s.EndDate),
		FormatDate(s.StartDate),
	))

	if err == nil {
		return Season{}, &OverlapError{With: conflictSeason}
	}

	if !errors.Is(err, sql.ErrNoRows) {
		return Season{}, fmt.Errorf("scan: %w", err)
	}

	season, err := scanSeason(tx.QueryRowContext(ctx,
		insertSeasonQuery,
		s.Name,
		FormatDate(s.StartDate),
		FormatDate(s.EndDate),
		st.now().Unix(),
	))
	if err != nil {
		return Season{}, fmt.Errorf("insert season: %w", err)
	}
	err = tx.Commit()
	if err != nil {
		return Season{}, fmt.Errorf("commit season: %w", err)
	}
	return season, nil

}

func (st *SeasonStore) List(ctx context.Context) ([]Season, error) {
	result := []Season{}
	rows, err := st.db.QueryContext(ctx, listSeasonsQuery)
	if err != nil {
		return nil, fmt.Errorf("queryContext: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		s, err := scanSeason(rows)
		if err != nil {
			return nil, fmt.Errorf("scanSeason: %w", err)
		}
		result = append(result, s)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return result, nil
}

// Current returns the season covering date. Only the calendar date counts,
// not the time of day, and both season bounds are inclusive. It returns
// ErrNotFound when no season covers the date.
func (st *SeasonStore) Current(ctx context.Context, date time.Time) (Season, error) {
	r := st.db.QueryRowContext(ctx, selectSeasonOverlappingQuery, FormatDate(date), FormatDate(date))
	s, err := scanSeason(r)
	if errors.Is(err, sql.ErrNoRows) {
		return Season{}, ErrNotFound
	}
	if err != nil {
		return Season{}, fmt.Errorf("current season for date %s: %w", FormatDate(date), err)
	}
	return s, nil
}

// Get returns the season with the given ID, or ErrNotFound.
func (st *SeasonStore) Get(ctx context.Context, id int64) (Season, error) {
	r := st.db.QueryRowContext(ctx, selectSeasonByIDQuery, id)
	s, err := scanSeason(r)
	if errors.Is(err, sql.ErrNoRows) {
		return Season{}, ErrNotFound
	}
	if err != nil {
		return Season{}, fmt.Errorf("get season %d: %w", id, err)
	}
	return s, nil
}

// scanSeason reads one season from a row selected with the columns
// id, name, start_date, end_date, created_at, in that order.
func scanSeason(r rowScanner) (Season, error) {
	var season Season
	var startDateString, endDateString string
	var createdAtInt int64

	err := r.Scan(
		&season.ID,
		&season.Name,
		&startDateString,
		&endDateString,
		&createdAtInt,
	)

	if err != nil {
		return Season{}, fmt.Errorf("scan season: %w", err)
	}
	parsedDate, err := ParseDate(startDateString)
	if err != nil {
		return Season{}, fmt.Errorf("parseDate(startDate): %w", err)
	}
	season.StartDate = parsedDate
	parsedDate, err = ParseDate(endDateString)
	if err != nil {
		return Season{}, fmt.Errorf("parseDate(endDate): %w", err)
	}
	season.EndDate = parsedDate

	season.CreatedAt = time.Unix(createdAtInt, 0)
	return season, nil
}
