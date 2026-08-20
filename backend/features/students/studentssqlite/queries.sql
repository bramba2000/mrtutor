-- name: GetStudentById :one
SELECT * FROM students WHERE id = ? LIMIT 1;

-- name: GetAllStudents :many
SELECT * FROM students;

-- name: GetDistinctSchools :many
SELECT DISTINCT school FROM students WHERE school IS NOT NULL AND school <> '' ORDER BY school;

-- name: GetDistinctStudyPrograms :many
SELECT DISTINCT study_program FROM students WHERE study_program IS NOT NULL AND study_program <> '' ORDER BY study_program;

-- name: GetDistinctClasses :many
SELECT DISTINCT class FROM students WHERE class IS NOT NULL AND class <> '' ORDER BY class;

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
