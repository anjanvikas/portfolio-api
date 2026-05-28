-- name: UpsertSocialLink :one
INSERT INTO social_link (name, url, sort_order)
VALUES ($1, $2, $3)
ON CONFLICT (name) DO UPDATE
SET url        = EXCLUDED.url,
    sort_order = EXCLUDED.sort_order
RETURNING *;

-- name: ListSocialLinks :many
SELECT * FROM social_link ORDER BY sort_order;
