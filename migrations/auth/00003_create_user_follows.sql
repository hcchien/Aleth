-- +goose Up
CREATE TABLE user_follows (
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    followee_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, followee_id),
    CHECK (follower_id <> followee_id)
);

CREATE INDEX idx_user_follows_followee_id ON user_follows (followee_id);
CREATE INDEX idx_user_follows_follower_id ON user_follows (follower_id);

-- +goose Down
DROP TABLE IF EXISTS user_follows;
