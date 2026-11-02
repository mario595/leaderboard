package main

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/testutil"
)

const testBaseURL = "https://leaderboard.example"

// create prints only the link, so it can be copied (or piped) as is.
var linkPattern = regexp.MustCompile(`^https://leaderboard\.example/invite/([A-Z2-7]{26})\n$`)

func TestInviteCommand_create(t *testing.T) {
	st := newTestInvitationStore(t)

	out, err := runInviteCmd(t, st, "create", "-uses", "10")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	m := linkPattern.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("create output = %q, want %s/invite/<code> on one line", out, testBaseURL)
	}
	inv := findTestInvitation(t, st, m[1])
	testutil.AssertEqual(t, "MaxUses", 10, inv.MaxUses)
	if !inv.ExpiresAt.IsZero() {
		t.Errorf("ExpiresAt = %v, want zero: no -expires means it never expires", inv.ExpiresAt)
	}
}

func TestInviteCommand_createWithExpiry(t *testing.T) {
	st := newTestInvitationStore(t)

	before := time.Now().Truncate(time.Second)
	out, err := runInviteCmd(t, st, "create", "-uses", "1", "-expires", "168h")
	after := time.Now()
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	m := linkPattern.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("create output = %q, want %s/invite/<code> on one line", out, testBaseURL)
	}
	// The command reads the real clock: the expiry is a week after some moment
	// during the call.
	got := findTestInvitation(t, st, m[1]).ExpiresAt
	week := 7 * 24 * time.Hour
	if got.Before(before.Add(week)) || got.After(after.Add(week)) {
		t.Errorf("ExpiresAt = %v, want between %v and %v", got, before.Add(week), after.Add(week))
	}
}

func TestInviteCommand_listEmpty(t *testing.T) {
	st := newTestInvitationStore(t)

	out, err := runInviteCmd(t, st, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if want := "no invitations\n"; out != want {
		t.Errorf("list output = %q, want %q", out, want)
	}
}

// One line per invitation, newest first:
// <code>  <uses>/<max> uses  expires <YYYY-MM-DD HH:MM> UTC|never[  revoked]
func TestInviteCommand_list(t *testing.T) {
	st := newTestInvitationStore(t)
	expiring := createTestInvitation(t, st, 5, time.Now().Add(48*time.Hour))
	revoked := createTestInvitation(t, st, 1, time.Time{})
	if err := st.Revoke(t.Context(), revoked.Code); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	open := createTestInvitation(t, st, 3, time.Time{})

	out, err := runInviteCmd(t, st, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	expires := expiring.ExpiresAt.UTC().Format("2006-01-02 15:04")
	want := open.Code + "  0/3 uses  expires never\n" +
		revoked.Code + "  0/1 uses  expires never  revoked\n" +
		expiring.Code + "  0/5 uses  expires " + expires + " UTC\n"
	if out != want {
		t.Errorf("list output =\n%s\nwant\n%s", out, want)
	}
}

func TestInviteCommand_revoke(t *testing.T) {
	st := newTestInvitationStore(t)
	inv := createTestInvitation(t, st, 1, time.Time{})
	other := createTestInvitation(t, st, 1, time.Time{})

	out, err := runInviteCmd(t, st, "revoke", "-code", inv.Code)
	if err != nil {
		t.Fatalf("revoke: %v", err)
	}

	if want := "revoked " + inv.Code + "\n"; out != want {
		t.Errorf("revoke output = %q, want %q", out, want)
	}
	if findTestInvitation(t, st, inv.Code).RevokedAt.IsZero() {
		t.Error("invitation not revoked")
	}
	// Positive control: only the given code.
	if !findTestInvitation(t, st, other.Code).RevokedAt.IsZero() {
		t.Error("the other invitation was revoked too")
	}
}

func TestInviteCommand_revokeUnknown(t *testing.T) {
	st := newTestInvitationStore(t)

	_, err := runInviteCmd(t, st, "revoke", "-code", "NOSUCHCODE")

	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("revoke unknown code: error = %v, want store.ErrNotFound", err)
	}
}

func TestInviteCommand_errors(t *testing.T) {
	testCases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{"no subcommand", nil, "usage"},
		{"unknown subcommand", []string{"nope"}, "nope"},
		{"unknown flag", []string{"create", "-bogus"}, "-bogus"},
		{"missing uses", []string{"create"}, "invalid invitation"},
		{"zero uses", []string{"create", "-uses", "0"}, "invalid invitation"},
		{"negative expiry", []string{"create", "-uses", "1", "-expires", "-1h"}, "-expires"},
		{"bad expiry", []string{"create", "-uses", "1", "-expires", "a week"}, "-expires"},
		{"revoke without code", []string{"revoke"}, "-code"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			st := newTestInvitationStore(t)

			_, err := runInviteCmd(t, st, tc.args...)
			if err == nil {
				t.Fatalf("inviteCommand(%q) succeeded, want an error containing %q", tc.args, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}

			// Nothing was created.
			list, err := st.List(t.Context())
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			testutil.AssertEqual(t, "invitations stored", 0, len(list))
		})
	}
}

func newTestInvitationStore(t *testing.T) *store.InvitationStore {
	t.Helper()
	return store.NewInvitationStore(newTestDB(t))
}

func createTestInvitation(t *testing.T, st *store.InvitationStore, maxUses int, expiresAt time.Time) store.Invitation {
	t.Helper()
	inv, err := st.Create(t.Context(), maxUses, expiresAt)
	if err != nil {
		t.Fatalf("create invitation: %v", err)
	}
	return inv
}

func findTestInvitation(t *testing.T, st *store.InvitationStore, code string) store.Invitation {
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
	t.Fatalf("invitation %q not stored", code)
	return store.Invitation{}
}

func runInviteCmd(t *testing.T, st *store.InvitationStore, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := inviteCommand(t.Context(), st, testBaseURL, args, &out)
	return out.String(), err
}
