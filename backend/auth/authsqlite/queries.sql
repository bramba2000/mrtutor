-- name: CreatePrincipal :one
-- CreatePrincipal creates a new principal in the database. Return the id of the newly created principal.
INSERT INTO users (username, email, password_hash, created_at)
    VALUES (:username, :email, :password_hash, :created_at)
    RETURNING id;

-- name: GetPrincipalByUsernameOrEmail :one
-- GetPrincipalByUsernameOrEmail retrieves a principal by username or email. Return the principal if found, otherwise return null.
SELECT * FROM principals WHERE username = :token OR email = :token;

-- name: CreateAuthSession :exec
-- CreateAuthSession creates a new authentication session for a principal. Return the id of the newly created session.
INSERT INTO sessions (user_id, id, created_at)
    VALUES (:user_id, :token_hash, :created_at);

-- name: RevokeSession :exec
-- RevokeSession revokes an authentication session
UPDATE sessions SET revoked_at = CURRENT_TIMESTAMP WHERE id = :token_hash;

-- name: GetAuthSessionByToken :one
-- GetAuthSessionByToken retrieves an authentication session by token.
SELECT * FROM sessions WHERE id = :token_hash;

-- name: GetPrincipalById :one
-- GetPrincipalById retrieves a principal by id.
SELECT * FROM principals WHERE id = :id;

-- name: DeleteExpiredSessions :exec
-- DeleteExpiredSessions deletes all revoked or expired sessions from the database.
-- Expired sessions are those that were created before the specified expiration time.
DELETE FROM sessions WHERE revoked_at IS NOT NULL OR created_at < :expiration_time;
