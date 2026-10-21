package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const upsertUserQuery = `
	INSERT INTO users (strava_id, first_name, last_name, access_token, refresh_token, token_expires_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(strava_id) DO UPDATE SET
		first_name = excluded.first_name,
		last_name = excluded.last_name,
		access_token = excluded.access_token,
		refresh_token = excluded.refresh_token,
		token_expires_at = excluded.token_expires_at,
		updated_at = excluded.updated_at
	RETURNING id, strava_id, first_name, last_name, access_token, refresh_token, token_expires_at, created_at, updated_at;
`

const getByStravaIDQuery = `
	SELECT id, strava_id, first_name, last_name, access_token, refresh_token, token_expires_at, created_at, updated_at
	FROM users
	WHERE strava_id = ?;
`

const getByIDQuery = `
	SELECT id, strava_id, first_name, last_name, access_token, refresh_token, token_expires_at, created_at, updated_at
	FROM users
	WHERE id = ?;
`

// User is a Strava user
type User struct {
	ID             int64
	StravaID       int64
	FirstName      string
	LastName       string
	AccessToken    string
	RefreshToken   string
	TokenExpiresAt time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// UserStore persists users in SQLite.
type UserStore struct {
	db  *sql.DB
	now func() time.Time
}

// NewUserStore returns a UserStore backed by db.
func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{
		db:  db,
		now: time.Now,
	}
}

// Upsert inserts the user, or updates it if its Strava ID already exists.
func (s *UserStore) Upsert(ctx context.Context, u User) (User, error) {
	now := s.now()

	r := s.db.QueryRowContext(
		ctx,
		upsertUserQuery,
		u.StravaID,
		u.FirstName,
		u.LastName,
		u.AccessToken,
		u.RefreshToken,
		u.TokenExpiresAt.Unix(),
		now.Unix(),
		now.Unix(),
	)

	user, err := scanUserRow(r)
	if err != nil {
		return User{}, fmt.Errorf("upsert user: %w", err)
	}
	return user, nil
}

func (s *UserStore) GetByStravaID(ctx context.Context, stravaID int64) (User, error) {
	r := s.db.QueryRowContext(ctx, getByStravaIDQuery, stravaID)
	user, err := scanUserRow(r)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user by strava id: %w", err)
	}

	return user, nil
}

// GetByID returns the user with the given internal id, or ErrNotFound.
func (s *UserStore) GetByID(ctx context.Context, id int64) (User, error) {
	r := s.db.QueryRowContext(ctx, getByIDQuery, id)
	user, err := scanUserRow(r)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user by id: %w", err)
	}
	return user, nil
}

func scanUserRow(r rowScanner) (User, error) {
	var user User
	var expiresAt, createdAt, updatedAt int64
	err := r.Scan(
		&user.ID,
		&user.StravaID,
		&user.FirstName,
		&user.LastName,
		&user.AccessToken,
		&user.RefreshToken,
		&expiresAt,
		&createdAt,
		&updatedAt,
	)

	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}

	user.TokenExpiresAt = time.Unix(expiresAt, 0)
	user.CreatedAt = time.Unix(createdAt, 0)
	user.UpdatedAt = time.Unix(updatedAt, 0)

	return user, nil
}
