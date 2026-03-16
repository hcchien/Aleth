-- +goose Up
ALTER TABLE posts
    ADD COLUMN kind         TEXT NOT NULL DEFAULT 'post'
        CHECK (kind IN ('post', 'note')),
    ADD COLUMN note_title   TEXT,
    ADD COLUMN note_cover   TEXT,
    ADD COLUMN note_summary TEXT;

CREATE INDEX idx_posts_notes_created ON posts (created_at DESC)
    WHERE deleted_at IS NULL AND kind = 'note' AND parent_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_posts_notes_created;
ALTER TABLE posts
    DROP COLUMN IF EXISTS kind,
    DROP COLUMN IF EXISTS note_title,
    DROP COLUMN IF EXISTS note_cover,
    DROP COLUMN IF EXISTS note_summary;
