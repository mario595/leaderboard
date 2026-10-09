-- +goose Up
CREATE TABLE seasons (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    start_date TEXT NOT NULL,
    end_date TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    CHECK (end_date > start_date)
);

-- +goose Down
DROP TABLE seasons;