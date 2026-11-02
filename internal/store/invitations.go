package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"
)

// Invitation is a sign-up link code that new athletes need to create an
// account. ExpiresAt is zero when it never expires, RevokedAt is zero when it
// has not been revoked.
type Invitation struct {
	ID                              int64
	Code                            string
	MaxUses                         int
	Uses                            int
	ExpiresAt, CreatedAt, RevokedAt time.Time
}

// InvitationStore reads and writes invitations.
type InvitationStore struct {
	db         DBTX
	now        func() time.Time
	randomText func() string
}

// NewInvitationStore returns an InvitationStore that uses db and the real clock.
func NewInvitationStore(db DBTX) *InvitationStore {
	return &InvitationStore{
		db:         db,
		now:        time.Now,
		randomText: rand.Text,
	}
}

const createInvitationQuery = `
	INSERT INTO invitations(code, max_uses, expires_at, created_at)
	VALUES (?, ?, ?, ?)
	RETURNING id, code, max_uses, uses, expires_at, created_at, revoked_at;
`

const listInvitationsQuery = `
	SELECT id, code, max_uses, uses, expires_at, created_at, revoked_at
	FROM invitations
	ORDER BY id DESC;
`

const revokeInvitationQuery = `
	UPDATE invitations
	SET revoked_at = COALESCE(revoked_at, ?)
	WHERE code = ?;
`

// Create stores a new invitation with a random code, usable maxUses times. A
// zero expiresAt means it never expires. maxUses below 1 or an expiry in the
// past returns an error wrapping ErrInvalidInvitation.
func (s *InvitationStore) Create(ctx context.Context, maxUses int, expiresAt time.Time) (Invitation, error) {
	if maxUses < 1 {
		return Invitation{}, fmt.Errorf("max uses %d: %w", maxUses, ErrInvalidInvitation)
	}
	var expiry any
	now := s.now().Unix()
	if !expiresAt.IsZero() {
		if expiresAt.Unix() <= now {
			return Invitation{}, fmt.Errorf("expires %v: %w", expiresAt, ErrInvalidInvitation)
		}
		expiry = expiresAt.Unix()
	}

	code := s.randomText()

	rows := s.db.QueryRowContext(ctx, createInvitationQuery, code, maxUses, expiry, now)
	inv, err := scanInvitationRow(rows)
	if err != nil {
		return Invitation{}, fmt.Errorf("create invitation: %w", err)
	}
	return inv, nil
}

// List returns every invitation, newest first.
func (s *InvitationStore) List(ctx context.Context) ([]Invitation, error) {
	var result = []Invitation{}
	rows, err := s.db.QueryContext(ctx, listInvitationsQuery)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		invitation, err := scanInvitationRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan invitation: %w", err)
		}
		result = append(result, invitation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan invitation: %w", err)
	}

	return result, nil
}

// Revoke marks the invitation with the given code as revoked, so it can no
// longer be used. Revoking it again keeps the first revocation time. An
// unknown code returns ErrNotFound.
func (s *InvitationStore) Revoke(ctx context.Context, code string) error {
	r, err := s.db.ExecContext(ctx, revokeInvitationQuery, s.now().Unix(), code)
	if err != nil {
		return fmt.Errorf("revoke invitation %s: %w", code, err)
	}
	a, err := r.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke invitation %s: %w", code, err)
	}
	if a == 0 {
		return ErrNotFound
	}
	return nil
}

// scanInvitationRow reads one invitation; NULL times become the zero time.
func scanInvitationRow(r rowScanner) (Invitation, error) {
	var invitation = Invitation{}
	var expiresAt sql.NullInt64
	var createdAt int64
	var revokedAt sql.NullInt64
	err := r.Scan(
		&invitation.ID,
		&invitation.Code,
		&invitation.MaxUses,
		&invitation.Uses,
		&expiresAt,
		&createdAt,
		&revokedAt,
	)
	if err != nil {
		return Invitation{}, err
	}
	if expiresAt.Valid {
		invitation.ExpiresAt = time.Unix(expiresAt.Int64, 0)
	}
	if revokedAt.Valid {
		invitation.RevokedAt = time.Unix(revokedAt.Int64, 0)
	}

	invitation.CreatedAt = time.Unix(createdAt, 0)

	return invitation, nil
}
