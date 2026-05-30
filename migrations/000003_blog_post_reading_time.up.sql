-- Persist reading_time_mins on the post itself. Until now the public read
-- queries computed it on the fly from the body (≈200 wpm, floored at 1 min).
-- The admin CRUD (SCRUM-66) must save it on every write, so it becomes a real
-- column and the read queries select it instead of recomputing — one source of
-- truth, no drift.
ALTER TABLE blog_post
  ADD COLUMN reading_time_mins integer NOT NULL DEFAULT 1;

-- Backfill existing rows with the same formula the old read queries used, so
-- nothing regresses between this migration and the first admin write.
UPDATE blog_post
SET reading_time_mins = GREATEST(1, CEIL(
    COALESCE(array_length(regexp_split_to_array(btrim(body), '\s+'), 1), 0)::numeric / 200
))::int;

-- The admin editor's "cover image URL" (SCRUM-66) is a plain URL field, not an
-- asset-library pick — there is no upload pipeline yet. Store it directly on the
-- post so reads and writes share one source of truth; the cover_asset_id column
-- stays for a future media library but is no longer used for blog covers.
ALTER TABLE blog_post
  ADD COLUMN cover_url text NOT NULL DEFAULT '';

-- Backfill from any cover asset already linked, so existing covers survive the
-- switch away from the asset join.
UPDATE blog_post p
SET cover_url = a.r2_key
FROM asset a
WHERE a.id = p.cover_asset_id;
