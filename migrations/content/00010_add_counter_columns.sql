-- +goose Up

-- Denormalized counters on posts for fast feed reads.
-- These are maintained asynchronously by the counter service via Pub/Sub.
-- See docs/architecture.md §12 for the consistency trade-off rationale.
ALTER TABLE posts
    ADD COLUMN comment_count  INT NOT NULL DEFAULT 0,
    ADD COLUMN reaction_count INT NOT NULL DEFAULT 0;

-- Article comment counter, maintained the same way.
ALTER TABLE articles
    ADD COLUMN comment_count INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE posts
    DROP COLUMN IF EXISTS comment_count,
    DROP COLUMN IF EXISTS reaction_count;

ALTER TABLE articles
    DROP COLUMN IF EXISTS comment_count;
