package store

import (
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/testutil"
)

// A code from crypto/rand.Text: 26 characters of the base32 alphabet.
var codePattern = regexp.MustCompile(`^[A-Z2-7]{26}$`)

func TestInvitationStore_create(t *testing.T) {
	st := newTestInvitationStore(t)
	created := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return created }
	expires := created.Add(7 * 24 * time.Hour)

	inv, err := st.Create(t.Context(), 10, expires)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if inv.ID == 0 {
		t.Error("ID is 0, want the stored row's ID")
	}
	if !codePattern.MatchString(inv.Code) {
		t.Errorf("Code = %q, want 26 base32 characters", inv.Code)
	}
	testutil.AssertEqual(t, "MaxUses", 10, inv.MaxUses)
	testutil.AssertEqual(t, "Uses", 0, inv.Uses)
	testutil.AssertTime(t, "CreatedAt", created, inv.CreatedAt)
	testutil.AssertTime(t, "ExpiresAt", expires, inv.ExpiresAt)
	if !inv.RevokedAt.IsZero() {
		t.Errorf("RevokedAt = %v, want zero (not revoked)", inv.RevokedAt)
	}
}

func TestInvitationStore_codesDiffer(t *testing.T) {
	st := newTestInvitationStore(t)

	a, err := st.Create(t.Context(), 1, time.Time{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	b, err := st.Create(t.Context(), 1, time.Time{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if a.Code == b.Code {
		t.Errorf("two invitations share the code %q", a.Code)
	}
}

func TestInvitationStore_createInvalid(t *testing.T) {
	for _, uses := range []int{0, -1} {
		st := newTestInvitationStore(t)

		_, err := st.Create(t.Context(), uses, time.Time{})

		if !errors.Is(err, ErrInvalidInvitation) {
			t.Errorf("Create(%d uses) error = %v, want ErrInvalidInvitation", uses, err)
		}
		list, err := st.List(t.Context())
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		testutil.AssertEqual(t, "invitations stored", 0, len(list))
	}
}

func TestInvitationStore_createExpired(t *testing.T) {
	now := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		expires time.Time
	}{
		{"in the past", now.Add(-time.Hour)},
		{"now", now},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := newTestInvitationStore(t)
			st.now = func() time.Time { return now }

			_, err := st.Create(t.Context(), 1, tt.expires)

			if !errors.Is(err, ErrInvalidInvitation) {
				t.Errorf("Create(expires %v) error = %v, want ErrInvalidInvitation", tt.expires, err)
			}
		})
	}

	// Positive control: one second later is valid.
	st := newTestInvitationStore(t)
	st.now = func() time.Time { return now }
	if _, err := st.Create(t.Context(), 1, now.Add(time.Second)); err != nil {
		t.Errorf("Create(expires in 1s) error = %v, want nil", err)
	}
}

// List returns every invitation, newest first, with what was stored: an
// expiry or none (the zero time), and no revocation yet.
func TestInvitationStore_list(t *testing.T) {
	st := newTestInvitationStore(t)
	now := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return now }
	expires := now.Add(48 * time.Hour)

	older, err := st.Create(t.Context(), 5, expires)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	newer, err := st.Create(t.Context(), 1, time.Time{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	list, err := st.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d invitations, want 2", len(list))
	}
	testutil.AssertEqual(t, "first is the newest", newer.Code, list[0].Code)
	testutil.AssertEqual(t, "second is the oldest", older.Code, list[1].Code)

	testutil.AssertEqual(t, "MaxUses", 5, list[1].MaxUses)
	testutil.AssertTime(t, "ExpiresAt", expires, list[1].ExpiresAt)
	if !list[0].ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt without expiry = %v, want zero", list[0].ExpiresAt)
	}
	if !list[1].RevokedAt.IsZero() {
		t.Errorf("RevokedAt = %v, want zero", list[1].RevokedAt)
	}
}

func TestInvitationStore_revoke(t *testing.T) {
	st := newTestInvitationStore(t)
	first := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return first }
	inv, err := st.Create(t.Context(), 3, time.Time{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	other, err := st.Create(t.Context(), 3, time.Time{})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := st.Revoke(t.Context(), inv.Code); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	// Revoking again later keeps the first revocation time.
	st.now = func() time.Time { return first.Add(time.Hour) }
	if err := st.Revoke(t.Context(), inv.Code); err != nil {
		t.Fatalf("second Revoke: %v", err)
	}

	got := findInvitation(t, st, inv.Code)
	testutil.AssertTime(t, "RevokedAt", first, got.RevokedAt)
	// Only that invitation.
	if r := findInvitation(t, st, other.Code).RevokedAt; !r.IsZero() {
		t.Errorf("the other invitation was revoked at %v", r)
	}
}

func TestInvitationStore_revokeUnknown(t *testing.T) {
	st := newTestInvitationStore(t)

	err := st.Revoke(t.Context(), "NOSUCHCODE")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Revoke(unknown) error = %v, want ErrNotFound", err)
	}
}

func newTestInvitationStore(t *testing.T) *InvitationStore {
	t.Helper()
	return NewInvitationStore(newTestDB(t))
}

func findInvitation(t *testing.T, st *InvitationStore, code string) Invitation {
	t.Helper()
	list, err := st.List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	for _, inv := range list {
		if inv.Code == code {
			return inv
		}
	}
	t.Fatalf("invitation %q not listed", code)
	return Invitation{}
}
