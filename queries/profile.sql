-- name: UpsertProfile :one
-- Singleton: keyed by email so re-running the seed updates the existing row.
INSERT INTO profile (name, headline, bio, location, email, resume_url, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (email) DO UPDATE
SET name       = EXCLUDED.name,
    headline   = EXCLUDED.headline,
    bio        = EXCLUDED.bio,
    location   = EXCLUDED.location,
    resume_url = EXCLUDED.resume_url,
    avatar_url = EXCLUDED.avatar_url,
    updated_at = now()
RETURNING *;

-- name: GetProfile :one
SELECT * FROM profile LIMIT 1;
