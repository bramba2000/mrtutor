-- +goose Up
-- This migration creates the "users" table and principal views, for the auth feature

CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash BLOB NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME
);

CREATE VIEW principals AS
SELECT id, username, email, password_hash, created_at, updated_at FROM users;

-- +goose Down
-- This migration drops the "users" table and principal views

DROP VIEW IF EXISTS principals;
DROP TABLE IF EXISTS users;
