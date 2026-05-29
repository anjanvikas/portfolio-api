-- name: UpsertProject :one
INSERT INTO project (
    slug, title, tagline, summary,
    body_overview, body_why_built, body_learning,
    cover_asset_id, repo_url, live_url,
    sort_order, featured, published_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (slug) DO UPDATE
SET title          = EXCLUDED.title,
    tagline        = EXCLUDED.tagline,
    summary        = EXCLUDED.summary,
    body_overview  = EXCLUDED.body_overview,
    body_why_built = EXCLUDED.body_why_built,
    body_learning  = EXCLUDED.body_learning,
    cover_asset_id = EXCLUDED.cover_asset_id,
    repo_url       = EXCLUDED.repo_url,
    live_url       = EXCLUDED.live_url,
    sort_order     = EXCLUDED.sort_order,
    featured       = EXCLUDED.featured,
    published_at   = EXCLUDED.published_at,
    updated_at     = now()
RETURNING *;

-- name: LinkProjectTag :exec
INSERT INTO project_tags (project_id, tag_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: ListProjects :many
SELECT * FROM project
WHERE published_at IS NOT NULL
ORDER BY sort_order, published_at DESC;

-- name: GetProjectBySlug :one
-- Powers the project detail page. Returns the full project (all three markdown
-- body sections, repo/live links, meta) plus the cover asset key and the
-- aggregated tag names so the page builds in a single round trip. Only
-- published projects are visible to the public site.
SELECT
    p.slug,
    p.title,
    p.tagline,
    p.summary,
    p.body_overview,
    p.body_why_built,
    p.body_learning,
    p.repo_url,
    p.live_url,
    p.published_at,
    a.r2_key AS cover_key,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM project p
LEFT JOIN asset a       ON a.id = p.cover_asset_id AND a.deleted_at IS NULL
LEFT JOIN project_tags pt ON pt.project_id = p.id
LEFT JOIN tag t         ON t.id = pt.tag_id
WHERE p.slug = sqlc.arg(slug)
  AND p.published_at IS NOT NULL
GROUP BY p.id, a.r2_key;

-- name: ListProjectCards :many
-- Powers the homepage "featured work" strip and the projects index. Joins the
-- cover asset (nullable) and aggregates the project's tag names into a single
-- text[] so the handler builds each card in one round trip. When featured_only
-- is true the result is limited to projects flagged for the homepage.
SELECT
    p.slug,
    p.title,
    p.summary,
    a.r2_key AS cover_key,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM project p
LEFT JOIN asset a       ON a.id = p.cover_asset_id AND a.deleted_at IS NULL
LEFT JOIN project_tags pt ON pt.project_id = p.id
LEFT JOIN tag t         ON t.id = pt.tag_id
WHERE p.published_at IS NOT NULL
  AND (NOT sqlc.arg(featured_only)::boolean OR p.featured = true)
GROUP BY p.id, a.r2_key
ORDER BY p.sort_order, p.published_at DESC
LIMIT sqlc.arg(row_limit);
