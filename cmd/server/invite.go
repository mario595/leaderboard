package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/mario595/leaderboard/internal/store"
)

func inviteCommand(ctx context.Context, st *store.InvitationStore, baseURL string, args []string, stdout io.Writer) error {
	const usage = "usage: server invite list | revoke -code C | create -uses N [-expires D]"
	if len(args) < 1 {
		return fmt.Errorf("incorrect usage. %s", usage)
	}
	switch args[0] {
	case "list":
		return listInvites(ctx, st, stdout)
	case "revoke":
		return revokeInvite(ctx, st, args[1:], stdout)
	case "create":
		return createInvite(ctx, st, baseURL, args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %s. %s", args[0], usage)
	}
}

func listInvites(ctx context.Context, st *store.InvitationStore, stdout io.Writer) error {
	lst, err := st.List(ctx)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	if len(lst) == 0 {
		_, _ = fmt.Fprintln(stdout, "no invitations")
	}

	for _, i := range lst {
		_, _ = fmt.Fprintln(stdout, formatInvitation(i))
	}

	return nil
}

func revokeInvite(ctx context.Context, st *store.InvitationStore, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("invite revoke", flag.ContinueOnError)
	fs.SetOutput(stdout)
	code := fs.String("code", "", "code (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *code == "" {
		return errors.New("-code is required")
	}

	if err := st.Revoke(ctx, *code); err != nil {
		return fmt.Errorf("revoke -code %s: %w", *code, err)
	}
	_, _ = fmt.Fprintf(stdout, "revoked %s\n", *code)

	return nil
}

func createInvite(ctx context.Context, st *store.InvitationStore, baseURL string, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("invite create", flag.ContinueOnError)
	fs.SetOutput(stdout)
	uses := fs.Int("uses", 0, "uses (required)")
	expires := fs.Duration("expires", 0, "expires")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *expires < 0 {
		return fmt.Errorf("invalid -expires %v: must not be negative", *expires)
	}
	var expiresAt time.Time
	if *expires != 0 {
		expiresAt = time.Now().Add(*expires)
	}

	i, err := st.Create(ctx, *uses, expiresAt)
	if err != nil {
		return fmt.Errorf("create invitation: %w", err)
	}
	_, _ = fmt.Fprintf(stdout, "%s/invite/%s\n", baseURL, i.Code)

	return nil
}

func formatInvitation(i store.Invitation) string {
	var expires = "never"
	if !i.ExpiresAt.IsZero() {
		expires = i.ExpiresAt.UTC().Format("2006-01-02 15:04 UTC")
	}

	var revoked string
	if !i.RevokedAt.IsZero() {
		revoked = "  revoked"
	}

	return fmt.Sprintf("%s  %d/%d uses  expires %s%s", i.Code, i.Uses, i.MaxUses, expires, revoked)
}
