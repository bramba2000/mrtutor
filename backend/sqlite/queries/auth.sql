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
