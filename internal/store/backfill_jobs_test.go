package store

import (
	"database/sql"
	"testing"
	"time"

	"github.com/mario595/leaderboard/internal/testutil"
)

func TestBackfillStore_enqueueAndNextDue(t *testing.T) {
	s, db := newTestBackfillStore(t)
	userID := createTestUser(t, db, 1001)
	seasonID := createTestSeason(t, NewSeasonStore(db), "Autumn", "2026-09-01", "2026-12-31").ID

	queued, err := s.Enqueue(t.Context(), userID, seasonID)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testutil.AssertEqual(t, "queued", true, queued)

	job, found, err := s.NextDue(t.Context())
	if err != nil || !found {
		t.Fatalf("NextDue: found=%v err=%v, want the queued job", found, err)
	}
	if job.ID == 0 {
		t.Error("ID is 0, want the stored row's ID")
	}
	testutil.AssertEqual(t, "UserID", userID, job.UserID)
	testutil.AssertEqual(t, "SeasonID", seasonID, job.SeasonID)
	testutil.AssertEqual(t, "Status", "pending", job.Status)
	testutil.AssertEqual(t, "Attempts", 0, job.Attempts)
}

// A job that is already waiting is not queued twice: signing up and running
// the CLI at the same time must not import the same season twice.
func TestBackfillStore_enqueuePendingIsANoOp(t *testing.T) {
	s, db := newTestBackfillStore(t)
	userID := createTestUser(t, db, 1001)
	seasonID := createTestSeason(t, NewSeasonStore(db), "Autumn", "2026-09-01", "2026-12-31").ID
	first := enqueueTestJob(t, s, userID, seasonID)
	// A retry is waiting: enqueueing again must not reset its attempts.
	notBefore := s.now().Add(time.Hour)
	if err := s.MarkRetry(t.Context(), first, "status 500", notBefore); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}

	queued, err := s.Enqueue(t.Context(), userID, seasonID)
	if err != nil {
		t.Fatalf("second Enqueue: %v", err)
	}

	testutil.AssertEqual(t, "queued", false, queued)
	testutil.AssertEqual(t, "jobs", 1, countRows(t, db, "backfill_jobs", "user_id", userID))
	r := readTestJob(t, db, first)
	testutil.AssertEqual(t, "attempts", 1, r.attempts)
	testutil.AssertEqual(t, "not_before", notBefore.Unix(), r.notBefore)
}

// A finished job (done or failed) is set back to pending, as new: this is how
// a season is imported again.
func TestBackfillStore_enqueueFinishedResetsIt(t *testing.T) {
	for _, finish := range []string{"done", "failed"} {
		t.Run(finish, func(t *testing.T) {
			s, db := newTestBackfillStore(t)
			userID := createTestUser(t, db, 1001)
			seasonID := createTestSeason(t, NewSeasonStore(db), "Autumn", "2026-09-01", "2026-12-31").ID
			id := enqueueTestJob(t, s, userID, seasonID)
			if err := s.MarkRetry(t.Context(), id, "status 500", s.now().Add(time.Hour)); err != nil {
				t.Fatalf("MarkRetry: %v", err)
			}
			var err error
			if finish == "done" {
				err = s.MarkDone(t.Context(), id)
			} else {
				err = s.MarkFailed(t.Context(), id, "strava: unauthorized")
			}
			if err != nil {
				t.Fatalf("mark %s: %v", finish, err)
			}

			queued, err := s.Enqueue(t.Context(), userID, seasonID)
			if err != nil {
				t.Fatalf("Enqueue: %v", err)
			}

			testutil.AssertEqual(t, "queued", true, queued)
			testutil.AssertEqual(t, "jobs", 1, countRows(t, db, "backfill_jobs", "user_id", userID))
			r := readTestJob(t, db, id)
			testutil.AssertEqual(t, "status", "pending", r.status)
			testutil.AssertEqual(t, "attempts", 0, r.attempts)
			testutil.AssertEqual(t, "last_error", "", r.lastError)
			testutil.AssertEqual(t, "not_before", int64(0), r.notBefore)
			testutil.AssertEqual(t, "done_at is NULL", false, r.doneAt.Valid)
			// And it is due at once.
			job, found, err := s.NextDue(t.Context())
			if err != nil || !found || job.ID != id {
				t.Errorf("NextDue: job %d found=%v err=%v, want job %d", job.ID, found, err, id)
			}
		})
	}
}

func TestBackfillStore_enqueueAll(t *testing.T) {
	s, db := newTestBackfillStore(t)
	seasons := NewSeasonStore(db)
	autumn := createTestSeason(t, seasons, "Autumn", "2026-09-01", "2026-12-31").ID
	spring := createTestSeason(t, seasons, "Spring", "2027-03-01", "2027-05-31").ID
	alice := createTestUser(t, db, 1001)
	bob := createTestUser(t, db, 1002)
	carol := createTestUser(t, db, 1003)
	// Alice already has a job waiting for autumn, and one for another season.
	enqueueTestJob(t, s, alice, autumn)
	enqueueTestJob(t, s, alice, spring)

	n, err := s.EnqueueAll(t.Context(), autumn)
	if err != nil {
		t.Fatalf("EnqueueAll: %v", err)
	}

	testutil.AssertEqual(t, "jobs queued", 2, n) // bob and carol
	for _, u := range []int64{alice, bob, carol} {
		testutil.AssertEqual(t, "autumn jobs of the user", 1, countJobs(t, db, u, autumn))
	}
	// The other season is untouched.
	testutil.AssertEqual(t, "spring jobs of bob", 0, countJobs(t, db, bob, spring))

	// Twice in a row: everyone is already waiting.
	n, err = s.EnqueueAll(t.Context(), autumn)
	if err != nil {
		t.Fatalf("second EnqueueAll: %v", err)
	}
	testutil.AssertEqual(t, "jobs queued the second time", 0, n)
}

func TestBackfillStore_nextDue(t *testing.T) {
	s, db := newTestBackfillStore(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	seasonID := createTestSeason(t, NewSeasonStore(db), "Autumn", "2026-09-01", "2026-12-31").ID

	if _, found, err := s.NextDue(t.Context()); err != nil || found {
		t.Fatalf("NextDue on an empty queue: found=%v err=%v, want nothing and no error", found, err)
	}

	waiting := enqueueTestJob(t, s, createTestUser(t, db, 1001), seasonID)
	done := enqueueTestJob(t, s, createTestUser(t, db, 1002), seasonID)
	failed := enqueueTestJob(t, s, createTestUser(t, db, 1003), seasonID)
	due := enqueueTestJob(t, s, createTestUser(t, db, 1004), seasonID)
	later := enqueueTestJob(t, s, createTestUser(t, db, 1005), seasonID)
	if err := s.MarkRetry(t.Context(), waiting, "rate limited", now.Add(time.Minute)); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}
	if err := s.MarkDone(t.Context(), done); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	if err := s.MarkFailed(t.Context(), failed, "strava: unauthorized"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	// Oldest due first: the waiting, done and failed jobs come before it but
	// are skipped.
	job, found, err := s.NextDue(t.Context())
	if err != nil || !found {
		t.Fatalf("NextDue: found=%v err=%v", found, err)
	}
	testutil.AssertEqual(t, "next due job", due, job.ID)

	// Once its time comes, the retry is due again, before the newer job.
	if err := s.MarkDone(t.Context(), due); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	now = now.Add(time.Minute)
	job, found, err = s.NextDue(t.Context())
	if err != nil || !found {
		t.Fatalf("NextDue after the retry time: found=%v err=%v", found, err)
	}
	testutil.AssertEqual(t, "next due job after the retry time", waiting, job.ID)
	testutil.AssertEqual(t, "Attempts", 1, job.Attempts)
	testutil.AssertTime(t, "NotBefore", now, job.NotBefore)

	if err := s.MarkDone(t.Context(), waiting); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	job, found, err = s.NextDue(t.Context())
	if err != nil || !found {
		t.Fatalf("NextDue for the last job: found=%v err=%v", found, err)
	}
	testutil.AssertEqual(t, "last job", later, job.ID)
}

func TestBackfillStore_markDoneRetryFailed(t *testing.T) {
	s, db := newTestBackfillStore(t)
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	seasonID := createTestSeason(t, NewSeasonStore(db), "Autumn", "2026-09-01", "2026-12-31").ID
	done := enqueueTestJob(t, s, createTestUser(t, db, 1001), seasonID)
	retried := enqueueTestJob(t, s, createTestUser(t, db, 1002), seasonID)
	failed := enqueueTestJob(t, s, createTestUser(t, db, 1003), seasonID)

	if err := s.MarkDone(t.Context(), done); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	retryAt := now.Add(15 * time.Minute)
	if err := s.MarkRetry(t.Context(), retried, "rate limited", retryAt); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}
	if err := s.MarkFailed(t.Context(), failed, "strava: unauthorized"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	d := readTestJob(t, db, done)
	testutil.AssertEqual(t, "done status", "done", d.status)
	testutil.AssertEqual(t, "done attempts", 0, d.attempts)
	testutil.AssertEqual(t, "done done_at", now.Unix(), d.doneAt.Int64)

	r := readTestJob(t, db, retried)
	testutil.AssertEqual(t, "retried status", "pending", r.status)
	testutil.AssertEqual(t, "retried attempts", 1, r.attempts)
	testutil.AssertEqual(t, "retried last_error", "rate limited", r.lastError)
	testutil.AssertEqual(t, "retried not_before", retryAt.Unix(), r.notBefore)
	testutil.AssertEqual(t, "retried done_at is NULL", false, r.doneAt.Valid)

	f := readTestJob(t, db, failed)
	testutil.AssertEqual(t, "failed status", "failed", f.status)
	testutil.AssertEqual(t, "failed attempts", 1, f.attempts)
	testutil.AssertEqual(t, "failed last_error", "strava: unauthorized", f.lastError)
	testutil.AssertEqual(t, "failed done_at", now.Unix(), f.doneAt.Int64)
}

// A member who leaves (deauthorization, #15) takes their jobs along.
func TestBackfillStore_deletingUserDeletesJobs(t *testing.T) {
	s, db := newTestBackfillStore(t)
	seasonID := createTestSeason(t, NewSeasonStore(db), "Autumn", "2026-09-01", "2026-12-31").ID
	leaving := createTestUser(t, db, 1001)
	staying := createTestUser(t, db, 1002)
	enqueueTestJob(t, s, leaving, seasonID)
	enqueueTestJob(t, s, staying, seasonID)

	if _, err := NewUserStore(db).DeleteByStravaID(t.Context(), 1001); err != nil {
		t.Fatalf("DeleteByStravaID: %v", err)
	}

	testutil.AssertEqual(t, "jobs of the deleted user", 0, countRows(t, db, "backfill_jobs", "user_id", leaving))
	testutil.AssertEqual(t, "jobs of the other user", 1, countRows(t, db, "backfill_jobs", "user_id", staying))
}

func TestBackfillStore_enqueueUnknownUser(t *testing.T) {
	s, db := newTestBackfillStore(t)
	seasonID := createTestSeason(t, NewSeasonStore(db), "Autumn", "2026-09-01", "2026-12-31").ID

	if _, err := s.Enqueue(t.Context(), 999, seasonID); err == nil {
		t.Error("Enqueue for an unknown user: got nil error, want the foreign key to reject it")
	}
}

func newTestBackfillStore(t *testing.T) (*BackfillStore, *sql.DB) {
	t.Helper()
	db := newTestDB(t)
	return NewBackfillStore(db), db
}

// enqueueTestJob queues a job and returns its ID.
func enqueueTestJob(t *testing.T, s *BackfillStore, userID, seasonID int64) int64 {
	t.Helper()
	queued, err := s.Enqueue(t.Context(), userID, seasonID)
	if err != nil || !queued {
		t.Fatalf("Enqueue(user %d, season %d): queued=%v err=%v", userID, seasonID, queued, err)
	}
	var id int64
	if err := s.db.QueryRowContext(t.Context(),
		"SELECT id FROM backfill_jobs WHERE user_id = ? AND season_id = ?", userID, seasonID,
	).Scan(&id); err != nil {
		t.Fatalf("select job id: %v", err)
	}
	return id
}

type testJobRow struct {
	status, lastError string
	attempts          int
	notBefore         int64
	doneAt            sql.NullInt64
}

// readTestJob reads the columns BackfillJob does not expose.
func readTestJob(t *testing.T, db *sql.DB, id int64) testJobRow {
	t.Helper()
	var r testJobRow
	if err := db.QueryRowContext(t.Context(),
		"SELECT status, last_error, attempts, not_before, done_at FROM backfill_jobs WHERE id = ?", id,
	).Scan(&r.status, &r.lastError, &r.attempts, &r.notBefore, &r.doneAt); err != nil {
		t.Fatalf("select job %d: %v", id, err)
	}
	return r
}

func countJobs(t *testing.T, db *sql.DB, userID, seasonID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(t.Context(),
		"SELECT COUNT(*) FROM backfill_jobs WHERE user_id = ? AND season_id = ?", userID, seasonID,
	).Scan(&n); err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	return n
}
