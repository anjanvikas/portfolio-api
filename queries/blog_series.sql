-- name: UpsertBlogSeries :one
INSERT INTO blog_series (slug, name, description)
VALUES ($1, $2, $3)
ON CONFLICT (slug) DO UPDATE
SET name        = EXCLUDED.name,
    description = EXCLUDED.description
RETURNING *;

-- name: GetBlogSeriesBySlug :one
SELECT * FROM blog_series WHERE slug = $1;

-- name: ListSeriesWithCounts :many
-- Powers GET /api/v1/series. Returns every series that has at least one
-- published post, with the count of published posts. The INNER JOIN drops
-- series whose posts are all still drafts (SCRUM-74 AC: exclude empty series).
SELECT
    s.id,
    s.slug,
    s.name,
    s.description,
    COUNT(p.id)::bigint AS post_count
FROM blog_series s
JOIN blog_post p ON p.series_id = s.id AND p.published_at IS NOT NULL
GROUP BY s.id
ORDER BY s.name;

-- name: ListPublishedPostsBySeriesSlug :many
-- Powers GET /api/v1/series/{slug}. Returns the series' published posts in
-- reading order so the frontend can build series nav (prev/next) and the
-- series landing page. Pairs with GetBlogSeriesBySlug for the series meta.
SELECT
    p.title,
    p.slug,
    p.series_order,
    p.published_at
FROM blog_post p
JOIN blog_series s ON s.id = p.series_id
WHERE s.slug = sqlc.arg(slug)
  AND p.published_at IS NOT NULL
ORDER BY p.series_order ASC;
