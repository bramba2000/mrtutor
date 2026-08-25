-- +goose Up
-- Create enrollments table linking tutors to students (many-to-many)
CREATE TABLE enrollments (
    id INTEGER PRIMARY KEY,
    tutor_id INTEGER NOT NULL REFERENCES tutors(id) ON DELETE CASCADE,
    student_id INTEGER NOT NULL REFERENCES students(id) ON DELETE CASCADE,

    created_at TIMESTAMP NOT NULL,

    UNIQUE (tutor_id, student_id)
);

-- +goose Down
DROP TABLE enrollments;
