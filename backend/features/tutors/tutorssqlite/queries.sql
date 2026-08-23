-- name: GetTutorById :one
SELECT * FROM tutors WHERE id = ? LIMIT 1;

-- name: GetAllTutors :many
SELECT * FROM tutors;

-- name: DeleteTutor :execrows
DELETE FROM tutors WHERE id = ?;

-- name: CreateTutor :one
INSERT INTO tutors (display_name, email, phone, about_me, user_id, created_at)
    VALUES (:display_name, :email, :phone, :about_me, :user_id, CURRENT_TIMESTAMP)
    RETURNING *;

-- name: UpdateTutor :one
UPDATE tutors
SET display_name = :display_name,
    email = :email,
    phone = :phone,
    about_me = :about_me,
    modified_at = CURRENT_TIMESTAMP
WHERE id = :id
RETURNING *;

-- name: GetTutorByUserID :one
SELECT * FROM tutors WHERE user_id = ? LIMIT 1;
