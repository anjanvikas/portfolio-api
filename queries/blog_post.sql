-- name: UpsertBlogPost :one
-- Used by the seed script. The admin CRUD (SCRUM-66) uses CreateBlogPost /
-- UpdateBlogPost instead so it can key on id and surface slug collisions.
INSERT INTO blog_post (slug, title, excerpt, body, cover_url, series_id, series_order, published_at, reading_time_mins)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (slug) DO UPDATE
SET title             = EXCLUDED.title,
    excerpt           = EXCLUDED.excerpt,
    body              = EXCLUDED.body,
    cover_url         = EXCLUDED.cover_url,
    series_id         = EXCLUDED.series_id,
    series_order      = EXCLUDED.series_order,
    published_at      = EXCLUDED.published_at,
    reading_time_mins = EXCLUDED.reading_time_mins,
    updated_at        = now()
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
-- its tag names, optional series (name + slug + order), and the stored reading
-- time (saved on every admin write). Newest first.
SELECT
    p.slug,
    p.title,
    p.excerpt,
    p.published_at,
    p.series_order,
    p.cover_url AS cover_key,
    bs.name   AS series_name,
    bs.slug   AS series_slug,
    p.reading_time_mins,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM blog_post p
LEFT JOIN blog_series bs   ON bs.id = p.series_id
LEFT JOIN blog_post_tags pt ON pt.blog_post_id = p.id
LEFT JOIN tag t            ON t.id = pt.tag_id
WHERE p.published_at IS NOT NULL
GROUP BY p.id, bs.name, bs.slug
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
    p.cover_url AS cover_key,
    bs.name   AS series_name,
    bs.slug   AS series_slug,
    p.reading_time_mins,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM blog_post p
LEFT JOIN blog_series bs   ON bs.id = p.series_id
LEFT JOIN blog_post_tags pt ON pt.blog_post_id = p.id
LEFT JOIN tag t            ON t.id = pt.tag_id
WHERE p.slug = sqlc.arg(slug)
  AND p.published_at IS NOT NULL
GROUP BY p.id, bs.name, bs.slug;

-- ===========================================================================
-- Admin CRUD (SCRUM-66) — these include drafts (published_at IS NULL), unlike
-- the public queries above.
-- ===========================================================================

-- name: ListAdminPosts :many
-- Powers the admin posts list table. Every post, drafts first (newest edited),
-- then published newest-first. Carries just what the table renders plus the
-- series name for context.
SELECT
    p.id,
    p.slug,
    p.title,
    p.published_at,
    p.reading_time_mins,
    p.updated_at,
    bs.name AS series_name
FROM blog_post p
LEFT JOIN blog_series bs ON bs.id = p.series_id
ORDER BY (p.published_at IS NULL) DESC, p.published_at DESC NULLS FIRST, p.updated_at DESC;

-- name: GetAdminPost :one
-- Loads a single post (draft or published) into the editor, with its series ref
-- and tag names.
SELECT
    p.id,
    p.slug,
    p.title,
    p.excerpt,
    p.body,
    p.cover_url,
    p.series_id,
    p.series_order,
    p.published_at,
    p.reading_time_mins,
    bs.name AS series_name,
    bs.slug AS series_slug,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM blog_post p
LEFT JOIN blog_series bs    ON bs.id = p.series_id
LEFT JOIN blog_post_tags pt ON pt.blog_post_id = p.id
LEFT JOIN tag t             ON t.id = pt.tag_id
WHERE p.id = sqlc.arg(id)
GROUP BY p.id, bs.name, bs.slug;

-- name: CreateBlogPost :one
-- Plain insert (not an upsert): a duplicate slug raises a unique-violation the
-- handler maps to 409, rather than silently overwriting another post.
INSERT INTO blog_post (slug, title, excerpt, body, cover_url, series_id, series_order, published_at, reading_time_mins)
VALUES (
    sqlc.arg(slug), sqlc.arg(title), sqlc.arg(excerpt), sqlc.arg(body),
    sqlc.arg(cover_url), sqlc.arg(series_id), sqlc.arg(series_order),
    sqlc.arg(published_at), sqlc.arg(reading_time_mins)
)
RETURNING *;

-- name: UpdateBlogPost :one
-- Updates every editable field by id. Deliberately does NOT touch published_at:
-- "Save Draft" must never unpublish a live post, and publishing is owned solely
-- by PublishBlogPost. Returns no rows when the id is unknown → handler 404s.
UPDATE blog_post
SET slug              = sqlc.arg(slug),
    title             = sqlc.arg(title),
    excerpt           = sqlc.arg(excerpt),
    body              = sqlc.arg(body),
    cover_url         = sqlc.arg(cover_url),
    series_id         = sqlc.arg(series_id),
    series_order      = sqlc.arg(series_order),
    reading_time_mins = sqlc.arg(reading_time_mins),
    updated_at        = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: PublishBlogPost :one
-- Promotes a draft to published. COALESCE preserves the original publish date
-- if the post was already live (re-publish is a no-op on the timestamp).
UPDATE blog_post
SET published_at = COALESCE(published_at, now()),
    updated_at   = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: DeleteBlogPost :execrows
-- Hard delete; blog_post_tags rows cascade. Returns the affected-row count so
-- the handler can 404 on an unknown id.
DELETE FROM blog_post WHERE id = sqlc.arg(id);

-- name: ClearBlogPostTags :exec
-- Drops all tag links for a post so the handler can re-link the submitted set
-- (tags are replaced wholesale on each save).
DELETE FROM blog_post_tags WHERE blog_post_id = sqlc.arg(blog_post_id);
