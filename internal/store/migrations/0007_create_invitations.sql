-- +goose Up
CREATE TABLE invitations (
    id INTEGER PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    max_uses INTEGER NOT NULL CHECK(max_uses > 0),
    uses INTEGER NOT NULL DEFAULT 0,
    expires_at INTEGER,
    created_at INTEGER NOT NULL,
    revoked_at INTEGER
);

ALTER TABLE users 
ADD COLUMN invitation_id INTEGER REFERENCES invitations(id);

-- +goose Down
ALTER TABLE users
DROP COLUMN invitation_id;

DROP TABLE invitations;
