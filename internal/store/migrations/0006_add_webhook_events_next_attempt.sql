-- +goose Up
ALTER TABLE webhook_events
ADD COLUMN next_attempt_at INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE webhook_events
DROP COLUMN next_attempt_at;
