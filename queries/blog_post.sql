-- name: UpsertBlogPost :one
INSERT INTO blog_post (slug, title, excerpt, body, cover_asset_id, series_id, series_order, published_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (slug) DO UPDATE
SET title          = EXCLUDED.title,
    excerpt        = EXCLUDED.excerpt,
    body           = EXCLUDED.body,
    cover_asset_id = EXCLUDED.cover_asset_id,
    series_id      = EXCLUDED.series_id,
    series_order   = EXCLUDED.series_order,
    published_at   = EXCLUDED.published_at,
    updated_at     = now()
RETURNING *;

-- name: LinkBlogPostTag :exec
INSERT INTO blog_post_tags (blog_post_id, tag_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListBlogPosts :many
SELECT * FROM blog_post
WHERE published_at IS NOT NULL
ORDER BY published_at DESC;
