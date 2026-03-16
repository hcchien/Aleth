-- +goose Up
CREATE TABLE article_comments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    article_id  UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    author_id   UUID NOT NULL,
    content     TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_article_comments_article_created_at
    ON article_comments (article_id, created_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_article_comments_author_id
    ON article_comments (author_id)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS article_comments;
