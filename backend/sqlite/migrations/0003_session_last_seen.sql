-- +goose Up
-- Track session activity so idle sessions expire independently of the absolute cap.
-- SQLite rejects CURRENT_TIMESTAMP as an ADD COLUMN default (it must be a constant),
-- hence the sentinel followed by a backfill from created_at.
ALTER TABLE sessions ADD COLUMN last_seen_at DATETIME NOT NULL DEFAULT '1970-01-01 00:00:00';
UPDATE sessions SET last_seen_at = created_at;
-- Logout now deletes the row outright; revocation is no longer a stored state.
ALTER TABLE sessions DROP COLUMN revoked_at;

-- +goose Down
ALTER TABLE sessions ADD COLUMN revoked_at DATETIME;
ALTER TABLE sessions DROP COLUMN last_seen_at;
