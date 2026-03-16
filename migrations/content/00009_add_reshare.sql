-- +goose Up
ALTER TABLE posts
    ADD COLUMN reshared_from_id UUID REFERENCES posts(id) ON DELETE SET NULL;

CREATE INDEX idx_posts_reshared_from ON posts (reshared_from_id)
    WHERE reshared_from_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_posts_reshared_from;
ALTER TABLE posts DROP COLUMN IF EXISTS reshared_from_id;
