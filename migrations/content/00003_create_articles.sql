-- +goose Up
CREATE TABLE articles (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id        UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    author_id       UUID NOT NULL,          -- references users.id (auth service)
    title           TEXT NOT NULL,
    slug            TEXT NOT NULL,
    content_md      TEXT,
    content_json    JSONB,                  -- rich text AST
    status          TEXT NOT NULL DEFAULT 'draft',   -- 'draft' | 'published' | 'unlisted'
    access_policy   TEXT NOT NULL DEFAULT 'public',  -- 'public' | 'members'
    min_trust_level SMALLINT NOT NULL DEFAULT 0,
    reach_score     NUMERIC NOT NULL DEFAULT 0,
    signature       JSONB,
    published_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_articles_board_slug ON articles (board_id, slug);
CREATE INDEX idx_articles_board_id ON articles (board_id, published_at DESC)
    WHERE status = 'published';
CREATE INDEX idx_articles_author_id ON articles (author_id);

-- +goose Down
DROP TABLE IF EXISTS articles;
