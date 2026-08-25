-- name: GetEnrollmentsByTutorID :many
SELECT * FROM enrollments WHERE tutor_id = ? ORDER BY id;

-- name: LinkTutorStudent :one
-- Tries to insert a new enrollment. If the (tutor_id, student_id) pair is
-- already enrolled, returns the existing row instead of erroring.
INSERT INTO enrollments (tutor_id, student_id, created_at)
    VALUES (?, ?, CURRENT_TIMESTAMP)
    ON CONFLICT (tutor_id, student_id) DO UPDATE SET tutor_id = excluded.tutor_id
    RETURNING *;
