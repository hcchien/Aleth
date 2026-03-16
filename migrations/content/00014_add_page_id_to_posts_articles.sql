-- +goose Up
ALTER TABLE posts
    ADD COLUMN page_id UUID REFERENCES fan_pages(id) ON DELETE SET NULL;

CREATE INDEX idx_posts_page_id ON posts (page_id, created_at DESC)
    WHERE page_id IS NOT NULL AND deleted_at IS NULL;

ALTER TABLE articles
    ADD COLUMN page_id UUID REFERENCES fan_pages(id) ON DELETE SET NULL;

CREATE INDEX idx_articles_page_id ON articles (page_id, published_at DESC)
    WHERE page_id IS NOT NULL AND status = 'published';

-- +goose Down
DROP INDEX IF EXISTS idx_articles_page_id;
ALTER TABLE articles DROP COLUMN IF EXISTS page_id;
DROP INDEX IF EXISTS idx_posts_page_id;
ALTER TABLE posts DROP COLUMN IF EXISTS page_id;
