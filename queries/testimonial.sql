-- name: UpsertTestimonial :one
INSERT INTO testimonial (author_name, author_role, author_company, quote, avatar_asset_id, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (author_name, quote) DO UPDATE
SET author_role     = EXCLUDED.author_role,
    author_company  = EXCLUDED.author_company,
    avatar_asset_id = EXCLUDED.avatar_asset_id,
    sort_order      = EXCLUDED.sort_order
RETURNING *;

-- name: ListTestimonials :many
SELECT * FROM testimonial ORDER BY sort_order;
