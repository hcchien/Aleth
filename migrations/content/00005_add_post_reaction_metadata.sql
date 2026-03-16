-- +goose Up
ALTER TABLE post_likes
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN source_ip INET;

CREATE INDEX idx_post_likes_source_ip ON post_likes (source_ip);

-- +goose Down
DROP INDEX IF EXISTS idx_post_likes_source_ip;
ALTER TABLE post_likes
    DROP COLUMN IF EXISTS source_ip,
    DROP COLUMN IF EXISTS updated_at;
