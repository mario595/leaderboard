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

const loginUserQuery = `
	UPDATE users
	SET first_name = ?, last_name = ?, access_token = ?, refresh_token = ?, token_expires_at = ?, updated_at = ?
	WHERE strava_id = ?
	RETURNING id, strava_id, first_name, last_name, access_token, refresh_token, token_expires_at, created_at, updated_at;
`

const insertUserQuery = `
	INSERT INTO users (invitation_id, strava_id, first_name, last_name, access_token, refresh_token, token_expires_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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

const updateTokensQuery = `
	UPDATE users
	SET access_token = ?,
		refresh_token = ?,
		token_expires_at = ?,
		updated_at = ?
	WHERE id = ?;
`

const deleteUserByStravaIDQuery = `
	DELETE FROM users
	WHERE strava_id = ?;
`

const deleteEventsByOwnerQuery = `
	DELETE FROM webhook_events
	WHERE owner_id = ?;
`

const useInvitationQuery = `
	UPDATE invitations
	SET uses = uses + 1
	WHERE code = ? AND uses < max_uses AND revoked_at IS NULL
	AND (expires_at IS NULL OR expires_at > ?)
	RETURNING id;
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

// Login updates the name and tokens of an existing athlete, found by Strava
// ID, and returns the stored user. A new athlete returns ErrNotFound: Login
// never creates a user (see SignUp).
func (s *UserStore) Login(ctx context.Context, u User) (User, error) {
	now := s.now()
	r := s.db.QueryRowContext(
		ctx,
		loginUserQuery,
		u.FirstName,
		u.LastName,
		u.AccessToken,
		u.RefreshToken,
		u.TokenExpiresAt.Unix(),
		now.Unix(),
		u.StravaID,
	)
	user, err := scanUserRow(r)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("login athlete %d: %w", u.StravaID, err)
	}

	return user, nil
}

// SignUp creates a new athlete with the invitation code, in one transaction:
// it takes one use of the invitation, then inserts the user. A code that is
// unknown, used up, revoked or expired returns an error wrapping
// ErrInvalidInvitation. If the insert fails (the athlete already exists), the
// rollback gives the use back.
func (s *UserStore) SignUp(ctx context.Context, u User, code string) (User, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, fmt.Errorf("sign up athlete %d: %w", u.StravaID, err)
	}
	defer func() { _ = tx.Rollback() }()

	now := s.now()
	var invitationID int64
	row := tx.QueryRowContext(ctx, useInvitationQuery, code, now.Unix())
	err = row.Scan(&invitationID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, fmt.Errorf("sign up athlete %d: %w", u.StravaID, ErrInvalidInvitation)
		}
		return User{}, fmt.Errorf("sign up athlete %d: %w", u.StravaID, err)
	}

	row = tx.QueryRowContext(
		ctx,
		insertUserQuery,
		invitationID,
		u.StravaID,
		u.FirstName,
		u.LastName,
		u.AccessToken,
		u.RefreshToken,
		u.TokenExpiresAt.Unix(),
		now.Unix(),
		now.Unix(),
	)
	user, err := scanUserRow(row)
	if err != nil {
		return User{}, fmt.Errorf("sign up athlete %d: %w", u.StravaID, err)
	}

	if err := tx.Commit(); err != nil {
		return User{}, fmt.Errorf("sign up athlete %d: %w", u.StravaID, err)
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

// UpdateTokens replaces the access token, refresh token and expiry of the
// user with the given internal ID, all together (Strava can rotate the
// refresh token on every refresh). It returns ErrNotFound if no such user
// exists: a silent no-op would lose the new tokens.
func (s *UserStore) UpdateTokens(ctx context.Context, userID int64, accessToken, refreshToken string, expiresAt time.Time) error {
	r, err := s.db.ExecContext(ctx, updateTokensQuery, accessToken, refreshToken, expiresAt.Unix(), s.now().Unix(), userID)
	if err != nil {
		return fmt.Errorf("update tokens for user %d: %w", userID, err)
	}
	rowsCount, err := r.RowsAffected()
	if err != nil {
		return fmt.Errorf("update tokens for user %d: %w", userID, err)
	}
	if rowsCount == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteByStravaID deletes everything stored for the athlete, in one
// transaction: their webhook events, then the user row, whose activities go
// with it through ON DELETE CASCADE. Used when the athlete deauthorizes the
// app. deleted reports whether a user existed; deleting an unknown athlete
// (a repeated event, or someone who never was a user) is not an error.
func (s *UserStore) DeleteByStravaID(ctx context.Context, stravaID int64) (deleted bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin transaction: delete athlete %d: %w", stravaID, err)
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, deleteEventsByOwnerQuery, stravaID)
	if err != nil {
		return false, fmt.Errorf("delete webhook events for athlete %d: %w", stravaID, err)
	}

	r, err := tx.ExecContext(ctx, deleteUserByStravaIDQuery, stravaID)
	if err != nil {
		return false, fmt.Errorf("delete athlete %d: %w", stravaID, err)
	}
	rows, err := r.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete athlete %d: %w", stravaID, err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("delete athlete %d: %w", stravaID, err)
	}
	return rows > 0, nil
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
