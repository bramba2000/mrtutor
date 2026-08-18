-- name: CreateStudent :one
INSERT INTO students (display_name, email, phone, school, study_program, class, birth_date, created_at)
    VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    RETURNING id;

-- name: GetStudentById :one
SELECT * FROM students WHERE id = ? LIMIT 1;

-- name: GetAllStudents :many
SELECT * FROM students;

-- name: UpdateStudent :one
UPDATE students SET display_name = ?, email = ?, phone = ?, school = ?, study_program = ?, class = ?, birth_date = ?, modified_at = ?
    WHERE id = ?
    RETURNING *;

-- name: DeleteStudent :exec
DELETE FROM students WHERE id = ?;
