-- +goose Up
-- oauth_states holds short-lived PKCE / CSRF state for social OAuth reputation flows.
-- Each row is consumed (deleted) on callback and cleaned up on expiry.
CREATE TABLE oauth_states (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider      TEXT        NOT NULL,    -- "twitter" | "facebook" | "instagram" | "linkedin"
    nonce         TEXT        NOT NULL,    -- random CSRF token used as OAuth `state` param
    code_verifier TEXT,                   -- PKCE code_verifier (Twitter only)
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at    TIMESTAMPTZ NOT NULL
);
CREATE UNIQUE INDEX idx_oauth_states_nonce ON oauth_states (nonce);
CREATE INDEX idx_oauth_states_user ON oauth_states (user_id);

-- +goose Down
DROP TABLE IF EXISTS oauth_states;
