-- name: UpsertTestimonial :one
-- Seed/idempotent path: keyed by (author_name, quote).
INSERT INTO testimonial (author_name, author_role, author_company, quote, avatar_asset_id, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (author_name, quote) DO UPDATE
SET author_role     = EXCLUDED.author_role,
    author_company  = EXCLUDED.author_company,
    avatar_asset_id = EXCLUDED.avatar_asset_id,
    sort_order      = EXCLUDED.sort_order
RETURNING *;

-- name: ListVisibleTestimonials :many
-- Public strip: only testimonials flagged visible, in display order.
SELECT * FROM testimonial WHERE visible ORDER BY sort_order;

-- name: ListTestimonials :many
-- Admin table: every testimonial regardless of visibility.
SELECT * FROM testimonial ORDER BY sort_order;

-- ---------------------------------------------------------------------------
-- Admin CRUD (SCRUM-68).
-- ---------------------------------------------------------------------------

-- name: GetTestimonial :one
SELECT * FROM testimonial WHERE id = $1;

-- name: CreateTestimonial :one
INSERT INTO testimonial (author_name, author_role, author_company, quote, visible, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateTestimonial :one
UPDATE testimonial
SET author_name    = $2,
    author_role    = $3,
    author_company = $4,
    quote          = $5,
    visible        = $6
WHERE id = $1
RETURNING *;

-- name: SetTestimonialVisibility :one
UPDATE testimonial SET visible = $2 WHERE id = $1 RETURNING *;

-- name: DeleteTestimonial :execrows
DELETE FROM testimonial WHERE id = $1;

-- name: NextTestimonialSortOrder :one
SELECT COALESCE(MAX(sort_order) + 1, 0)::int AS next FROM testimonial;
