-- name: UpsertTag :one
INSERT INTO tag (slug, name)
VALUES ($1, $2)
ON CONFLICT (slug) DO UPDATE
SET name = EXCLUDED.name
RETURNING *;

-- name: ListTags :many
SELECT * FROM tag ORDER BY name;
