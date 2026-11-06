-- +goose Up
CREATE TABLE backfill_jobs (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    season_id INTEGER NOT NULL REFERENCES seasons(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    not_before INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    done_at INTEGER,
    UNIQUE(user_id, season_id)
);

CREATE INDEX backfill_jobs_status_idx ON backfill_jobs(status, id);

-- +goose Down
DROP INDEX backfill_jobs_status_idx;
DROP TABLE backfill_jobs;
