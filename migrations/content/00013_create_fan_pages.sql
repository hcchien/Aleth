-- +goose Up
CREATE TABLE fan_pages (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                TEXT        NOT NULL,
    name                TEXT        NOT NULL,
    description         TEXT,
    avatar_url          TEXT,
    cover_url           TEXT,
    category            TEXT        NOT NULL DEFAULT 'general',
    ap_enabled          BOOLEAN     NOT NULL DEFAULT false,
    -- Content & comment access policy (mirrors boards table exactly)
    default_access      TEXT        NOT NULL DEFAULT 'public',    -- 'public' | 'members'
    min_trust_level     SMALLINT    NOT NULL DEFAULT 0,           -- min trust to post
    comment_policy      TEXT        NOT NULL DEFAULT 'public',    -- 'public' | 'members'
    min_comment_trust   SMALLINT    NOT NULL DEFAULT 0,           -- min trust to comment
    require_vcs         JSONB       NOT NULL DEFAULT '[]',        -- VC requirements for posting
    require_comment_vcs JSONB       NOT NULL DEFAULT '[]',        -- VC requirements for commenting
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_fan_pages_slug ON fan_pages (slug);

CREATE TABLE page_members (
    page_id   UUID        NOT NULL REFERENCES fan_pages(id) ON DELETE CASCADE,
    user_id   UUID        NOT NULL,
    role      TEXT        NOT NULL DEFAULT 'editor',  -- 'admin' | 'editor'
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (page_id, user_id)
);

CREATE INDEX idx_page_members_user_id ON page_members (user_id);

CREATE TABLE page_followers (
    page_id     UUID        NOT NULL REFERENCES fan_pages(id) ON DELETE CASCADE,
    user_id     UUID        NOT NULL,
    followed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (page_id, user_id)
);

CREATE INDEX idx_page_followers_page_id ON page_followers (page_id);

-- +goose Down
DROP TABLE IF EXISTS page_followers;
DROP TABLE IF EXISTS page_members;
DROP TABLE IF EXISTS fan_pages;
