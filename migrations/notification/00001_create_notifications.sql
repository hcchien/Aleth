-- +goose Up

CREATE TABLE notifications (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL,          -- recipient
    type         TEXT        NOT NULL,          -- 'reply', 'reshare', 'comment', 'reaction'
    actor_id     UUID        NOT NULL,          -- who triggered the notification
    entity_type  TEXT        NOT NULL,          -- 'post', 'comment'
    entity_id    UUID        NOT NULL,          -- the post or comment ID
    read         BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_user_unread
    ON notifications (user_id, created_at DESC)
    WHERE read = FALSE;

CREATE INDEX idx_notifications_user_all
    ON notifications (user_id, created_at DESC);

-- +goose Down

DROP INDEX IF EXISTS idx_notifications_user_all;
DROP INDEX IF EXISTS idx_notifications_user_unread;
DROP TABLE IF EXISTS notifications;
