-- +goose Up
-- Create students table
CREATE TABLE students (
    id INTEGER PRIMARY KEY,
    display_name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    school TEXT,
    study_program TEXT,
    class TEXT,
    birth_date DATE,

    created_at TIMESTAMP NOT NULL,
    modified_at TIMESTAMP
);

-- +goose Down
DROP TABLE students;
