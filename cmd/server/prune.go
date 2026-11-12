package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/mario595/leaderboard/internal/store"
	"github.com/mario595/leaderboard/internal/strava"
)

// pruneUsers is what prune-revoked needs from the user store.
type pruneUsers interface {
	List(ctx context.Context) ([]store.User, error)
	DeleteByStravaID(ctx context.Context, stravaID int64) (bool, error)
}

// accessCheck asks Strava whether it still accepts the app's authorization
// for u (strava.Client.CheckAccess in production).
type accessCheck func(ctx context.Context, u store.User) error

// pruneCommand finds the members whose Strava authorization is gone (they
// revoked the app, or deleted their account) and, with -delete, deletes them
// like a deauthorization event would. It exists for after a restore: a
// backup older than a member's departure brings their data back, and nothing
// in the restored database remembers that they left; Strava does.
//
// Only strava.ErrUnauthorized counts as revoked. Any other error (rate
// limit, network, Strava down) is reported and the member is kept. When
// every member is rejected, -delete refuses: that pattern means the app's
// own credentials are wrong, not that the whole group left.
func pruneCommand(ctx context.Context, users pruneUsers, check accessCheck, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("prune-revoked", flag.ContinueOnError)
	fs.SetOutput(stdout)
	del := fs.Bool("delete", false, "delete the revoked members (default: only list them)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	members, err := users.List(ctx)
	if err != nil {
		return fmt.Errorf("list members: %w", err)
	}

	var revoked []store.User
	failed := 0
	for _, u := range members {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := check(ctx, u)
		switch {
		case err == nil:
		case errors.Is(err, strava.ErrUnauthorized):
			revoked = append(revoked, u)
			_, _ = fmt.Fprintf(stdout, "revoked: %s\n", describeMember(u))
		default:
			failed++
			_, _ = fmt.Fprintf(stdout, "error: %s: %v\n", describeMember(u), err)
		}
	}

	deleted := 0
	if *del {
		if len(revoked) > 0 && len(revoked) == len(members) {
			return errors.New("every member was rejected by Strava: check STRAVA_CLIENT_ID and STRAVA_CLIENT_SECRET; nothing deleted")
		}
		for _, u := range revoked {
			if _, err := users.DeleteByStravaID(ctx, u.StravaID); err != nil {
				return fmt.Errorf("delete %s: %w", describeMember(u), err)
			}
			deleted++
			_, _ = fmt.Fprintf(stdout, "deleted: %s\n", describeMember(u))
		}
	}

	_, _ = fmt.Fprintf(stdout, "checked %d, revoked %d, errors %d, deleted %d\n", len(members), len(revoked), failed, deleted)
	if !*del && len(revoked) > 0 {
		_, _ = fmt.Fprintln(stdout, "dry run: run with -delete to delete the revoked members")
	}
	return nil
}

// describeMember names a member in the command's output, without tokens.
func describeMember(u store.User) string {
	return fmt.Sprintf("#%d %s %s (Strava %d)", u.ID, u.FirstName, u.LastName, u.StravaID)
}
