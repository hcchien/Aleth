-- +goose Up
ALTER TABLE article_comments
    ADD COLUMN parent_id UUID REFERENCES article_comments(id) ON DELETE CASCADE;

CREATE INDEX idx_article_comments_parent_id
    ON article_comments (parent_id, created_at ASC)
    WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_article_comments_parent_id;
ALTER TABLE article_comments DROP COLUMN IF EXISTS parent_id;
