-- name: GetHomeContent :one
SELECT * FROM home_content WHERE id = 1;

-- name: UpsertHomeContent :one
INSERT INTO home_content (id, open_to_roles, content)
VALUES (1, $1, $2)
ON CONFLICT (id) DO UPDATE
SET open_to_roles = EXCLUDED.open_to_roles,
    content       = EXCLUDED.content,
    updated_at    = now()
RETURNING *;
