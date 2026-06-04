-- SCRUM-69 (F14 SEO & OG images): persist the generated per-post OG image URL
-- so the public metadata API can return it without regenerating, and so the
-- generator is idempotent (regenerate = clear column).
ALTER TABLE blog_post
  ADD COLUMN og_image_url text NOT NULL DEFAULT '';
