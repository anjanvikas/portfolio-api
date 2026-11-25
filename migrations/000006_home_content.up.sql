CREATE TABLE home_content (
  id            SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  open_to_roles BOOLEAN NOT NULL DEFAULT true,
  content       JSONB   NOT NULL DEFAULT '{}'::jsonb,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO home_content (id) VALUES (1);
