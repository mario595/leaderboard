-- +goose Up
CREATE TABLE activities (
    strava_id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    sport_type TEXT NOT NULL,
    start_local TEXT NOT NULL,
    day TEXT NOT NULL,
    moving_time INTEGER NOT NULL,
    distance REAL NOT NULL,
    workout_type INTEGER NOT NULL DEFAULT 0,
    description TEXT NOT NULL DEFAULT '',
    points INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE INDEX activities_user_day_idx ON activities(user_id, day);

-- +goose Down
DROP INDEX activities_user_day_idx;
DROP TABLE activities;
