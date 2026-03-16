-- +goose Up
ALTER TABLE post_likes
    ADD COLUMN emotion TEXT NOT NULL DEFAULT 'like';

ALTER TABLE post_likes
    ADD CONSTRAINT chk_post_likes_emotion
    CHECK (emotion IN ('like', 'love', 'haha', 'wow', 'sad', 'angry'));

CREATE INDEX idx_post_likes_post_emotion ON post_likes (post_id, emotion);

-- +goose Down
DROP INDEX IF EXISTS idx_post_likes_post_emotion;
ALTER TABLE post_likes DROP CONSTRAINT IF EXISTS chk_post_likes_emotion;
ALTER TABLE post_likes DROP COLUMN IF EXISTS emotion;
