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

-- name: UpdateProfile :one
-- Admin edit of the singleton profile row (SCRUM-68). Updated by id, which the
-- handler reads from GetProfile first.
UPDATE profile
SET name       = $2,
    headline   = $3,
    bio        = $4,
    location   = $5,
    email      = $6,
    resume_url = $7,
    avatar_url = $8,
    updated_at = now()
WHERE id = $1
RETURNING *;
