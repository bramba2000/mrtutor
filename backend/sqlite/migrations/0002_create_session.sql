-- +goose Up
-- Create a session store table
CREATE TABLE sessions (
    id BLOB PRIMARY KEY, --sha256 hash of the session token
    user_id INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked_at DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) WITHOUT ROWID;

CREATE INDEX idx_sessions_user_id ON sessions(user_id);

-- +goose Down
-- Drop the session store table
DROP TABLE IF EXISTS sessions;
