-- +goose Up
CREATE TABLE webhook_events (
    id INTEGER PRIMARY KEY,
    object_type TEXT NOT NULL,
    object_id INTEGER NOT NULL,
    aspect_type TEXT NOT NULL,
    owner_id INTEGER NOT NULL,
    event_time INTEGER NOT NULL,
    updates TEXT NOT NULL DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    received_at INTEGER NOT NULL,
    processed_at INTEGER,
    UNIQUE(object_type, object_id, aspect_type, event_time)
);

CREATE INDEX webhook_events_status_idx ON webhook_events(status, id);

-- +goose Down
DROP INDEX webhook_events_status_idx;
DROP TABLE webhook_events;
