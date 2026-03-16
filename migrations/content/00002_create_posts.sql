-- +goose Up
CREATE TABLE posts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id       UUID NOT NULL,              -- references users.id (auth service)
    parent_id       UUID REFERENCES posts(id),  -- NULL for root posts
    root_id         UUID REFERENCES posts(id),  -- thread root node
    content         TEXT NOT NULL,
    media_ids       UUID[] NOT NULL DEFAULT '{}',
    reach_score     NUMERIC NOT NULL DEFAULT 0,
    signature       JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ                 -- soft delete
);

CREATE INDEX idx_posts_author_id ON posts (author_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_posts_parent_id ON posts (parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX idx_posts_root_id ON posts (root_id) WHERE root_id IS NOT NULL;
CREATE INDEX idx_posts_created_at ON posts (created_at DESC) WHERE deleted_at IS NULL;

CREATE TABLE post_likes (
    post_id         UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS post_likes;
DROP TABLE IF EXISTS posts;
