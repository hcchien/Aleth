-- +goose Up

-- remote_following: Aleth users following remote AP actors (e.g. Threads, Mastodon).
-- When a local user follows a remote actor, we send a Follow activity and record it here.
-- The `accepted` flag is set to true when the remote sends Accept(Follow) back.
CREATE TABLE remote_following (
    id                 UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    local_username     TEXT        NOT NULL,
    actor_url          TEXT        NOT NULL,
    inbox_url          TEXT        NOT NULL,
    follow_activity_id TEXT        NOT NULL,
    accepted           BOOLEAN     NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (local_username, actor_url)
);
CREATE INDEX idx_remote_following_username ON remote_following (local_username);

-- remote_posts: incoming federated Create(Note) activities pushed to a local user's inbox.
-- activity_id is the AP activity URL used as a deduplication key.
CREATE TABLE remote_posts (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    activity_id     TEXT        NOT NULL UNIQUE,
    actor_url       TEXT        NOT NULL,
    local_recipient TEXT        NOT NULL,
    content         TEXT        NOT NULL,
    published_at    TIMESTAMPTZ NOT NULL,
    raw_activity    JSONB       NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_remote_posts_recipient ON remote_posts (local_recipient, published_at DESC);

---- create above / drop below ----

DROP TABLE IF EXISTS remote_posts;
DROP TABLE IF EXISTS remote_following;
