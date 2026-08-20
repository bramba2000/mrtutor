-- name: GetTutorById :one
SELECT * FROM tutors WHERE id = ? LIMIT 1;

-- name: GetAllTutors :many
SELECT * FROM tutors;

-- name: DeleteTutor :execrows
DELETE FROM tutors WHERE id = ?;

-- name: SaveTutor :one
-- This query will try to insert a new tutor record. If a record with the same id
-- already exists, it will update the existing record instead.
INSERT INTO tutors (id, display_name, email, phone, about_me, created_at)
    VALUES (NULLIF(:id, 0), :display_name, :email, :phone, :about_me, CURRENT_TIMESTAMP)
    ON CONFLICT (id) DO UPDATE SET
        display_name = EXCLUDED.display_name,
        email = EXCLUDED.email,
        phone = EXCLUDED.phone,
        about_me = EXCLUDED.about_me,
        modified_at = CURRENT_TIMESTAMP
    RETURNING *;
