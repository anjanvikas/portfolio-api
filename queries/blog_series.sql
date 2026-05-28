-- name: UpsertBlogSeries :one
INSERT INTO blog_series (slug, name, description)
VALUES ($1, $2, $3)
ON CONFLICT (slug) DO UPDATE
SET name        = EXCLUDED.name,
    description = EXCLUDED.description
RETURNING *;

-- name: GetBlogSeriesBySlug :one
SELECT * FROM blog_series WHERE slug = $1;
