-- name: UpsertProject :one
INSERT INTO project (
    slug, title, tagline, summary,
    body_overview, body_why_built, body_learning,
    cover_asset_id, repo_url, live_url,
    sort_order, published_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
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
