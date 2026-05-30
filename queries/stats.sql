-- name: GetAdminStats :one
-- Dashboard counts for the admin overview (GET /api/v1/admin/stats). There is
-- no status column on blog_post: a post is "published" when published_at IS NOT
-- NULL and a "draft" otherwise. Projects are surfaced as a single total.
SELECT
    (SELECT COUNT(*) FROM blog_post)                                AS total_posts,
    (SELECT COUNT(*) FROM blog_post WHERE published_at IS NOT NULL) AS published_posts,
    (SELECT COUNT(*) FROM blog_post WHERE published_at IS NULL)     AS draft_posts,
    (SELECT COUNT(*) FROM project)                                  AS total_projects;
