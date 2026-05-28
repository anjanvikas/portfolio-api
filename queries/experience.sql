-- name: UpsertExperience :one
INSERT INTO experience (company, role, location, start_date, end_date, description, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (company, role, start_date) DO UPDATE
SET location    = EXCLUDED.location,
    end_date    = EXCLUDED.end_date,
    description = EXCLUDED.description,
    sort_order  = EXCLUDED.sort_order
RETURNING *;

-- name: ListExperience :many
SELECT * FROM experience ORDER BY sort_order DESC, start_date DESC;
