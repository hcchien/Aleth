-- Phase 3: remote follower tracking and outbound delivery queue.

-- remote_followers stores which remote AP actors follow a local Aleth user.
-- inbox_url is cached from the remote actor's AP profile and used for delivery.
CREATE TABLE remote_followers (
    id             UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    local_username TEXT        NOT NULL,
    actor_url      TEXT        NOT NULL,  -- e.g. https://mastodon.social/users/bob
    inbox_url      TEXT        NOT NULL,  -- e.g. https://mastodon.social/users/bob/inbox
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (local_username, actor_url)
);
CREATE INDEX idx_remote_followers_username ON remote_followers (local_username);

-- delivery_queue holds outbound ActivityPub activities waiting to be POSTed.
-- status: 'pending' | 'done' | 'failed'
CREATE TABLE delivery_queue (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    local_username  TEXT        NOT NULL,
    target_inbox    TEXT        NOT NULL,
    activity_json   JSONB       NOT NULL,
    attempts        INT         NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error      TEXT,
    status          TEXT        NOT NULL DEFAULT 'pending',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- Index only on pending items to keep the worker sweep fast.
CREATE INDEX idx_delivery_queue_pending
    ON delivery_queue (next_attempt_at)
    WHERE status = 'pending';

---- create above / drop below ----

DROP TABLE IF EXISTS delivery_queue;
DROP TABLE IF EXISTS remote_followers;
