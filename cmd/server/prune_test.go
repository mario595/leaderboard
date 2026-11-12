package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
	"github.com/mario595/leaderboard/internal/testutil"
)

// fakeAccess answers CheckAccess for each member by Strava ID; members not
// in the map are accepted. It records whom it checked.
type fakeAccess struct {
	errs    map[int64]error
	checked []int64
}

func (f *fakeAccess) check(_ context.Context, u store.User) error {
	f.checked = append(f.checked, u.StravaID)
	return f.errs[u.StravaID]
}

func TestPruneCommand_dryRunDeletesNothing(t *testing.T) {
	db := newTestDB(t)
	createPruneMembers(t, db)
	access := &fakeAccess{errs: map[int64]error{1002: revokedErr()}}

	out, err := runPruneCmd(t, db, access)
	if err != nil {
		t.Fatalf("prune-revoked: %v", err)
	}

	want := "revoked: #2 Bea Ruiz (Strava 1002)\n" +
		"checked 3, revoked 1, errors 0, deleted 0\n" +
		"dry run: run with -delete to delete the revoked members\n"
	testutil.AssertEqual(t, "output", want, out)
	assertIDs(t, "members left", []int64{1001, 1002, 1003}, memberStravaIDs(t, db))
	assertIDs(t, "checked", []int64{1001, 1002, 1003}, access.checked)
}

// Two members accepted and one rejected: a command that deleted a fixed
// member, or every member, would fail here.
func TestPruneCommand_deletesOnlyTheRevoked(t *testing.T) {
	db := newTestDB(t)
	createPruneMembers(t, db)
	access := &fakeAccess{errs: map[int64]error{1002: revokedErr()}}

	out, err := runPruneCmd(t, db, access, "-delete")
	if err != nil {
		t.Fatalf("prune-revoked -delete: %v", err)
	}

	want := "revoked: #2 Bea Ruiz (Strava 1002)\n" +
		"deleted: #2 Bea Ruiz (Strava 1002)\n" +
		"checked 3, revoked 1, errors 0, deleted 1\n"
	testutil.AssertEqual(t, "output", want, out)
	assertIDs(t, "members left", []int64{1001, 1003}, memberStravaIDs(t, db))
}

// Only an explicit rejection proves a member left. A rate limit or a network
// error says nothing about them: they stay.
func TestPruneCommand_otherErrorsKeepTheMember(t *testing.T) {
	db := newTestDB(t)
	createPruneMembers(t, db)
	access := &fakeAccess{errs: map[int64]error{
		1001: fmt.Errorf("strava GET /athlete: %w", &strava.RateLimitError{ShortUsage: 201}),
		1003: errors.New("strava GET /athlete: status 500"),
	}}

	out, err := runPruneCmd(t, db, access, "-delete")
	if err != nil {
		t.Fatalf("prune-revoked -delete: %v", err)
	}

	if !strings.Contains(out, "error: #1 Ana Gil (Strava 1001): ") || !strings.Contains(out, "error: #3 Carla Sanz (Strava 1003): strava GET /athlete: status 500\n") {
		t.Errorf("output does not report both errors:\n%s", out)
	}
	if !strings.HasSuffix(out, "checked 3, revoked 0, errors 2, deleted 0\n") {
		t.Errorf("output does not end with the summary:\n%s", out)
	}
	assertIDs(t, "members left", []int64{1001, 1002, 1003}, memberStravaIDs(t, db))
}

// Every member rejected looks like wrong client credentials, not like the
// whole group leaving: nothing is deleted.
func TestPruneCommand_refusesWhenEveryMemberIsRejected(t *testing.T) {
	db := newTestDB(t)
	createPruneMembers(t, db)
	access := &fakeAccess{errs: map[int64]error{1001: revokedErr(), 1002: revokedErr(), 1003: revokedErr()}}

	_, err := runPruneCmd(t, db, access, "-delete")

	if err == nil || !strings.Contains(err.Error(), "STRAVA_CLIENT_SECRET") {
		t.Fatalf("prune-revoked -delete with every member rejected: error = %v, want one naming STRAVA_CLIENT_SECRET", err)
	}
	assertIDs(t, "members left", []int64{1001, 1002, 1003}, memberStravaIDs(t, db))
}

// The dry run lists them all anyway: it is how the operator sees the problem.
func TestPruneCommand_dryRunListsEveryRejectedMember(t *testing.T) {
	db := newTestDB(t)
	createPruneMembers(t, db)
	access := &fakeAccess{errs: map[int64]error{1001: revokedErr(), 1002: revokedErr(), 1003: revokedErr()}}

	out, err := runPruneCmd(t, db, access)
	if err != nil {
		t.Fatalf("prune-revoked: %v", err)
	}

	if !strings.Contains(out, "checked 3, revoked 3, errors 0, deleted 0\n") {
		t.Errorf("output:\n%s", out)
	}
}

func TestPruneCommand_noMembers(t *testing.T) {
	db := newTestDB(t)
	access := &fakeAccess{}

	out, err := runPruneCmd(t, db, access, "-delete")
	if err != nil {
		t.Fatalf("prune-revoked -delete: %v", err)
	}

	testutil.AssertEqual(t, "output", "checked 0, revoked 0, errors 0, deleted 0\n", out)
}

func TestPruneCommand_unknownFlag(t *testing.T) {
	db := newTestDB(t)

	_, err := runPruneCmd(t, db, &fakeAccess{}, "-bogus")

	if err == nil || !strings.Contains(err.Error(), "-bogus") {
		t.Errorf("prune-revoked -bogus: error = %v, want one naming the flag", err)
	}
}

// revokedErr is what CheckAccess returns for a member who left: wrapped, so
// a command comparing with == instead of errors.Is fails.
func revokedErr() error {
	return fmt.Errorf("strava GET /athlete: %w", strava.ErrUnauthorized)
}

// createPruneMembers stores three members (internal IDs 1 to 3, Strava IDs
// 1001 to 1003).
func createPruneMembers(t *testing.T, db *sql.DB) {
	t.Helper()
	users := store.NewUserStore(db)
	for _, u := range []store.User{
		{StravaID: 1001, FirstName: "Ana", LastName: "Gil"},
		{StravaID: 1002, FirstName: "Bea", LastName: "Ruiz"},
		{StravaID: 1003, FirstName: "Carla", LastName: "Sanz"},
	} {
		u.AccessToken, u.RefreshToken = "a", "r"
		if _, err := users.Upsert(t.Context(), u); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
}

func memberStravaIDs(t *testing.T, db *sql.DB) []int64 {
	t.Helper()
	users, err := store.NewUserStore(db).List(t.Context())
	if err != nil {
		t.Fatalf("list users: %v", err)
	}
	ids := []int64{}
	for _, u := range users {
		ids = append(ids, u.StravaID)
	}
	slices.Sort(ids)
	return ids
}

func assertIDs(t *testing.T, name string, want, got []int64) {
	t.Helper()
	if !slices.Equal(want, got) {
		t.Errorf("%s: want %v, got %v", name, want, got)
	}
}

func runPruneCmd(t *testing.T, db *sql.DB, access *fakeAccess, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := pruneCommand(t.Context(), store.NewUserStore(db), access.check, args, &out)
	return out.String(), err
}
