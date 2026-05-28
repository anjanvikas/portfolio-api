-- Initial schema for portfolio (SCRUM-9 / SCRUM-44).
-- 9 entities from the data model (SCRUM-6) plus two M2M join tables.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------------------------------------------------------------------------
-- profile: singleton row with the site owner's identity.
-- ---------------------------------------------------------------------------
CREATE TABLE profile (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text        NOT NULL,
    headline    text        NOT NULL,
    bio         text        NOT NULL DEFAULT '',
    location    text        NOT NULL DEFAULT '',
    email       text        NOT NULL UNIQUE,
    resume_url  text,
    avatar_url  text,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- social_link: ordered list of named external URLs (github, linkedin, ...).
-- ---------------------------------------------------------------------------
CREATE TABLE social_link (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text        NOT NULL UNIQUE,
    url        text        NOT NULL,
    sort_order integer     NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX social_link_sort_order_idx ON social_link (sort_order);

-- ---------------------------------------------------------------------------
-- experience: work-history rows.
-- ---------------------------------------------------------------------------
CREATE TABLE experience (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    company     text        NOT NULL,
    role        text        NOT NULL,
    location    text        NOT NULL DEFAULT '',
    start_date  date        NOT NULL,
    end_date    date,
    description text        NOT NULL DEFAULT '',
    sort_order  integer     NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (company, role, start_date)
);
CREATE INDEX experience_sort_order_idx ON experience (sort_order DESC);

-- ---------------------------------------------------------------------------
-- asset: uploaded files (R2). Soft delete only.
-- ---------------------------------------------------------------------------
CREATE TABLE asset (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    filename   text        NOT NULL,
    mime_type  text        NOT NULL,
    size_bytes bigint      NOT NULL,
    r2_key     text        NOT NULL UNIQUE,
    width      integer,
    height     integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX asset_deleted_at_idx ON asset (deleted_at);

-- ---------------------------------------------------------------------------
-- project: portfolio case study. Body lives in three markdown sections
-- (overview / why I built this / learning journey) per the project-detail
-- wireframe.
-- ---------------------------------------------------------------------------
CREATE TABLE project (
    id                uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug              text        NOT NULL UNIQUE,
    title             text        NOT NULL,
    tagline           text        NOT NULL DEFAULT '',
    summary           text        NOT NULL DEFAULT '',
    body_overview     text        NOT NULL DEFAULT '',
    body_why_built    text        NOT NULL DEFAULT '',
    body_learning     text        NOT NULL DEFAULT '',
    cover_asset_id    uuid        REFERENCES asset(id) ON DELETE SET NULL,
    repo_url          text,
    live_url          text,
    sort_order        integer     NOT NULL DEFAULT 0,
    published_at      timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX project_published_at_idx ON project (published_at DESC NULLS LAST);
CREATE INDEX project_sort_order_idx   ON project (sort_order);

-- ---------------------------------------------------------------------------
-- blog_series: optional grouping for multi-part posts.
-- ---------------------------------------------------------------------------
CREATE TABLE blog_series (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        text        NOT NULL UNIQUE,
    name        text        NOT NULL,
    description text        NOT NULL DEFAULT '',
    created_at  timestamptz NOT NULL DEFAULT now()
);

-- ---------------------------------------------------------------------------
-- blog_post: markdown post; optionally part of a series.
-- ---------------------------------------------------------------------------
CREATE TABLE blog_post (
    id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug           text        NOT NULL UNIQUE,
    title          text        NOT NULL,
    excerpt        text        NOT NULL DEFAULT '',
    body           text        NOT NULL DEFAULT '',
    cover_asset_id uuid        REFERENCES asset(id) ON DELETE SET NULL,
    series_id      uuid        REFERENCES blog_series(id) ON DELETE SET NULL,
    series_order   integer,
    published_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT blog_post_series_order_chk
      CHECK ((series_id IS NULL AND series_order IS NULL)
          OR (series_id IS NOT NULL AND series_order IS NOT NULL))
);
CREATE INDEX blog_post_published_at_idx ON blog_post (published_at DESC NULLS LAST);
CREATE INDEX blog_post_series_idx       ON blog_post (series_id, series_order);

-- ---------------------------------------------------------------------------
-- tag: shared between projects and blog posts (many-to-many).
-- ---------------------------------------------------------------------------
CREATE TABLE tag (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text        NOT NULL UNIQUE,
    name       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE project_tags (
    project_id uuid NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    tag_id     uuid NOT NULL REFERENCES tag(id)     ON DELETE CASCADE,
    PRIMARY KEY (project_id, tag_id)
);
CREATE INDEX project_tags_tag_idx ON project_tags (tag_id);

CREATE TABLE blog_post_tags (
    blog_post_id uuid NOT NULL REFERENCES blog_post(id) ON DELETE CASCADE,
    tag_id       uuid NOT NULL REFERENCES tag(id)       ON DELETE CASCADE,
    PRIMARY KEY (blog_post_id, tag_id)
);
CREATE INDEX blog_post_tags_tag_idx ON blog_post_tags (tag_id);

-- ---------------------------------------------------------------------------
-- testimonial: optional ordered quotes.
-- ---------------------------------------------------------------------------
CREATE TABLE testimonial (
    id               uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    author_name      text        NOT NULL,
    author_role      text        NOT NULL DEFAULT '',
    author_company   text        NOT NULL DEFAULT '',
    quote            text        NOT NULL,
    avatar_asset_id  uuid        REFERENCES asset(id) ON DELETE SET NULL,
    sort_order       integer     NOT NULL DEFAULT 0,
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (author_name, quote)
);
CREATE INDEX testimonial_sort_order_idx ON testimonial (sort_order);
