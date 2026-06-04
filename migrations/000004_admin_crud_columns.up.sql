-- SCRUM-68 (F13 admin CRUD) needs two schema additions.

-- 1. Project covers move to a plain URL column, mirroring blog_post in 000003.
-- The admin editor's image picker (SCRUM-67) returns a public asset URL that is
-- stored directly on the row, so reads and writes share one source of truth.
-- cover_asset_id stays for a possible future media library but is no longer used
-- for project covers; the public read queries now select cover_url.
ALTER TABLE project
  ADD COLUMN cover_url text NOT NULL DEFAULT '';

-- Backfill from any cover asset already linked, so existing covers survive the
-- switch away from the asset join.
UPDATE project p
SET cover_url = a.r2_key
FROM asset a
WHERE a.id = p.cover_asset_id;

-- 2. Testimonials gain an explicit visibility flag for the admin toggle. Default
-- true so every existing testimonial stays public; the public /testimonials
-- endpoint now filters on it.
ALTER TABLE testimonial
  ADD COLUMN visible boolean NOT NULL DEFAULT true;
CREATE INDEX testimonial_visible_idx ON testimonial (visible);
