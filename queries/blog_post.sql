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

-- name: ListPublishedPostCards :many
-- Powers GET /api/v1/posts and the /blog list. One row per published post with
-- its tag names, optional series (name + slug + order), and a computed reading
-- time (≈200 wpm over the markdown body, floored at 1 min). Newest first.
SELECT
    p.slug,
    p.title,
    p.excerpt,
    p.published_at,
    p.series_order,
    a.r2_key  AS cover_key,
    bs.name   AS series_name,
    bs.slug   AS series_slug,
    GREATEST(1, CEIL(
        COALESCE(array_length(regexp_split_to_array(btrim(p.body), '\s+'), 1), 0)::numeric / 200
    ))::int AS reading_time_mins,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM blog_post p
LEFT JOIN asset a          ON a.id = p.cover_asset_id AND a.deleted_at IS NULL
LEFT JOIN blog_series bs   ON bs.id = p.series_id
LEFT JOIN blog_post_tags pt ON pt.blog_post_id = p.id
LEFT JOIN tag t            ON t.id = pt.tag_id
WHERE p.published_at IS NOT NULL
GROUP BY p.id, a.r2_key, bs.name, bs.slug
ORDER BY p.published_at DESC
LIMIT sqlc.arg(row_limit);

-- name: GetPublishedPostBySlug :one
-- Powers GET /api/v1/posts/{slug}. Full post body plus the same tag/series/
-- reading-time fields as the card query. Series siblings (prev/next, part X of
-- Y) are resolved separately via ListPublishedPostsBySeriesSlug. Published only.
SELECT
    p.slug,
    p.title,
    p.excerpt,
    p.body,
    p.published_at,
    p.series_order,
    a.r2_key  AS cover_key,
    bs.name   AS series_name,
    bs.slug   AS series_slug,
    GREATEST(1, CEIL(
        COALESCE(array_length(regexp_split_to_array(btrim(p.body), '\s+'), 1), 0)::numeric / 200
    ))::int AS reading_time_mins,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM blog_post p
LEFT JOIN asset a          ON a.id = p.cover_asset_id AND a.deleted_at IS NULL
LEFT JOIN blog_series bs   ON bs.id = p.series_id
LEFT JOIN blog_post_tags pt ON pt.blog_post_id = p.id
LEFT JOIN tag t            ON t.id = pt.tag_id
WHERE p.slug = sqlc.arg(slug)
  AND p.published_at IS NOT NULL
GROUP BY p.id, a.r2_key, bs.name, bs.slug;
