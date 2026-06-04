-- name: UpsertProject :one
-- Seed/idempotent path: keyed by slug so a re-seed updates the existing row.
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

-- name: ClearProjectTags :exec
DELETE FROM project_tags WHERE project_id = $1;

-- name: ListProjects :many
SELECT * FROM project
WHERE published_at IS NOT NULL
ORDER BY sort_order, published_at DESC;

-- name: GetProjectBySlug :one
-- Powers the project detail page. Returns the full project (all three markdown
-- body sections, repo/live links, meta) plus the cover URL and the aggregated
-- tag names so the page builds in a single round trip. Only published projects
-- are visible to the public site.
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
    p.cover_url AS cover_key,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM project p
LEFT JOIN project_tags pt ON pt.project_id = p.id
LEFT JOIN tag t           ON t.id = pt.tag_id
WHERE p.slug = sqlc.arg(slug)
  AND p.published_at IS NOT NULL
GROUP BY p.id;

-- name: ListProjectCards :many
-- Powers the homepage "featured work" strip and the projects index. Aggregates
-- the project's tag names into a single text[] so the handler builds each card
-- in one round trip. When featured_only is true the result is limited to
-- projects flagged for the homepage.
SELECT
    p.slug,
    p.title,
    p.summary,
    p.cover_url AS cover_key,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM project p
LEFT JOIN project_tags pt ON pt.project_id = p.id
LEFT JOIN tag t           ON t.id = pt.tag_id
WHERE p.published_at IS NOT NULL
  AND (NOT sqlc.arg(featured_only)::boolean OR p.featured = true)
GROUP BY p.id
ORDER BY p.sort_order, p.published_at DESC
LIMIT sqlc.arg(row_limit);

-- ---------------------------------------------------------------------------
-- Admin CRUD (SCRUM-68). These see drafts as well as published rows.
-- ---------------------------------------------------------------------------

-- name: ListAdminProjects :many
-- The admin projects table: every project, drafts first then by sort order.
SELECT id, slug, title, sort_order, featured, published_at, updated_at
FROM project
ORDER BY (published_at IS NOT NULL), sort_order, updated_at DESC;

-- name: GetAdminProject :one
-- Loads one project (any status) into the editor, with its tag names.
SELECT
    p.id,
    p.slug,
    p.title,
    p.tagline,
    p.summary,
    p.body_overview,
    p.body_why_built,
    p.body_learning,
    p.cover_url,
    p.repo_url,
    p.live_url,
    p.featured,
    p.sort_order,
    p.published_at,
    COALESCE(
        array_agg(t.name ORDER BY t.name) FILTER (WHERE t.id IS NOT NULL),
        '{}'
    )::text[] AS tags
FROM project p
LEFT JOIN project_tags pt ON pt.project_id = p.id
LEFT JOIN tag t           ON t.id = pt.tag_id
WHERE p.id = sqlc.arg(id)
GROUP BY p.id;

-- name: CreateProject :one
-- New projects are always created as drafts (published_at NULL).
INSERT INTO project (
    slug, title, tagline, summary,
    body_overview, body_why_built, body_learning,
    cover_url, repo_url, live_url, featured, sort_order
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id;

-- name: UpdateProject :one
-- Saves the editable fields by id. Deliberately does NOT touch published_at, so
-- saving never unpublishes a live project (the publish endpoint owns that).
UPDATE project
SET slug           = $2,
    title          = $3,
    tagline        = $4,
    summary        = $5,
    body_overview  = $6,
    body_why_built = $7,
    body_learning  = $8,
    cover_url      = $9,
    repo_url       = $10,
    live_url       = $11,
    featured       = $12,
    updated_at     = now()
WHERE id = $1
RETURNING id;

-- name: PublishProject :one
-- Sets published_at to now() the first time; re-publishing keeps the original.
UPDATE project
SET published_at = COALESCE(published_at, now()),
    updated_at   = now()
WHERE id = $1
RETURNING id;

-- name: DeleteProject :execrows
-- Hard delete; project_tags rows cascade.
DELETE FROM project WHERE id = $1;

-- name: NextProjectSortOrder :one
-- The sort_order to give a new project so it lands at the end of the list.
SELECT COALESCE(MAX(sort_order) + 1, 0)::int AS next FROM project;
