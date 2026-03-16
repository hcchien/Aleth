-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE boards (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id            UUID NOT NULL,          -- references users.id (auth service)
    name                TEXT NOT NULL,
    description         TEXT,
    cover_image_id      UUID,
    default_access      TEXT NOT NULL DEFAULT 'public',
    min_trust_level     SMALLINT NOT NULL DEFAULT 0,
    comment_policy      TEXT NOT NULL DEFAULT 'public',
    min_comment_trust   SMALLINT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_boards_owner_id ON boards (owner_id);

CREATE TABLE board_subscribers (
    board_id        UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL,          -- references users.id (auth service)
    subscribed_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (board_id, user_id)
);

-- +goose Down
DROP TABLE IF EXISTS board_subscribers;
DROP TABLE IF EXISTS boards;
