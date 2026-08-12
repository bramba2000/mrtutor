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
INSERT INTO sessions (user_id, id, created_at, last_seen_at)
    VALUES (:user_id, :token_hash, :created_at, :last_seen_at);

-- name: TouchAuthSession :exec
-- TouchAuthSession refreshes a session's activity timestamp.
UPDATE sessions SET last_seen_at = :last_seen_at WHERE id = :token_hash;

-- name: DeleteAuthSession :exec
-- DeleteAuthSession removes a single session (logout).
DELETE FROM sessions WHERE id = :token_hash;

-- name: GetAuthSessionByToken :one
-- GetAuthSessionByToken retrieves an authentication session by token.
SELECT * FROM sessions WHERE id = :token_hash;

-- name: GetPrincipalById :one
-- GetPrincipalById retrieves a principal by id.
SELECT * FROM principals WHERE id = :id;

-- name: DeleteExpiredSessions :exec
-- DeleteExpiredSessions deletes sessions past the absolute cap or the inactivity window.
DELETE FROM sessions WHERE created_at < :created_before OR last_seen_at < :last_seen_before;
