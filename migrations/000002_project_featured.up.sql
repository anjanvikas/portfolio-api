-- Add an explicit "featured" flag to projects so the admin can promote/demote
-- a project to the homepage strip without renumbering sort_order.
ALTER TABLE project ADD COLUMN featured boolean NOT NULL DEFAULT false;

-- Partial index: the homepage query only ever asks for featured rows.
CREATE INDEX project_featured_idx ON project (sort_order) WHERE featured;
