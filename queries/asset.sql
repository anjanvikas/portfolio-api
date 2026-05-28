-- name: UpsertAsset :one
INSERT INTO asset (filename, mime_type, size_bytes, r2_key, width, height)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (r2_key) DO UPDATE
SET filename   = EXCLUDED.filename,
    mime_type  = EXCLUDED.mime_type,
    size_bytes = EXCLUDED.size_bytes,
    width      = EXCLUDED.width,
    height     = EXCLUDED.height
RETURNING *;

-- name: SoftDeleteAsset :exec
UPDATE asset SET deleted_at = now() WHERE id = $1;

-- name: ListAssets :many
SELECT * FROM asset WHERE deleted_at IS NULL ORDER BY created_at DESC;
