-- name: UpsertExperience :one
-- Seed/idempotent path: keyed by (company, role, start_date).
INSERT INTO experience (company, role, location, start_date, end_date, description, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (company, role, start_date) DO UPDATE
SET location    = EXCLUDED.location,
    end_date    = EXCLUDED.end_date,
    description = EXCLUDED.description,
    sort_order  = EXCLUDED.sort_order
RETURNING *;

-- name: ListExperience :many
-- Display + admin order: highest sort_order first, newest start as the tiebreak.
SELECT * FROM experience ORDER BY sort_order DESC, start_date DESC;

-- ---------------------------------------------------------------------------
-- Admin CRUD (SCRUM-68).
-- ---------------------------------------------------------------------------

-- name: GetExperience :one
SELECT * FROM experience WHERE id = $1;

-- name: CreateExperience :one
INSERT INTO experience (company, role, location, start_date, end_date, description, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdateExperience :one
UPDATE experience
SET company     = $2,
    role        = $3,
    location    = $4,
    start_date  = $5,
    end_date    = $6,
    description = $7
WHERE id = $1
RETURNING *;

-- name: SetExperienceSortOrder :exec
-- One step of a drag-to-reorder save: assign a row its new sort_order.
UPDATE experience SET sort_order = $2 WHERE id = $1;

-- name: DeleteExperience :execrows
DELETE FROM experience WHERE id = $1;

-- name: NextExperienceSortOrder :one
-- The sort_order to give a new entry so it lands at the top (newest) of the list.
SELECT COALESCE(MAX(sort_order) + 1, 0)::int AS next FROM experience;
