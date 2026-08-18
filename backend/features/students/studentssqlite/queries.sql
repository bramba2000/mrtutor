-- name: GetStudentById :one
SELECT * FROM students WHERE id = ? LIMIT 1;

-- name: GetAllStudents :many
SELECT * FROM students;

-- name: DeleteStudent :execrows
DELETE FROM students WHERE id = ?;

-- name: SaveStudent :one
-- This query will try to insert a new student record. If a record with the same id
-- already exists, it will update the existing record instead.
INSERT INTO students (id, display_name, email, phone, school, study_program, class, birth_date, created_at)
    VALUES (NULLIF(:id, 0), :display_name, :email, :phone, :school, :study_program, :class, :birth_date, CURRENT_TIMESTAMP)
    ON CONFLICT (id) DO UPDATE SET
        display_name = EXCLUDED.display_name,
        email = EXCLUDED.email,
        phone = EXCLUDED.phone,
        school = EXCLUDED.school,
        study_program = EXCLUDED.study_program,
        class = EXCLUDED.class,
        birth_date = EXCLUDED.birth_date,
        modified_at = CURRENT_TIMESTAMP
    RETURNING *;
