DROP INDEX IF EXISTS testimonial_visible_idx;
ALTER TABLE testimonial DROP COLUMN IF EXISTS visible;
ALTER TABLE project DROP COLUMN IF EXISTS cover_url;
