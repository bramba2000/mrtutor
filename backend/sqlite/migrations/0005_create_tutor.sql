-- +goose Up
-- Create tutors table
CREATE TABLE tutors (
    id INTEGER PRIMARY KEY,
    display_name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    about_me TEXT,
    user_id INTEGER NOT NULL UNIQUE REFERENCES users(id),

    created_at TIMESTAMP NOT NULL,
    modified_at TIMESTAMP
);

-- +goose Down
DROP TABLE tutors;
