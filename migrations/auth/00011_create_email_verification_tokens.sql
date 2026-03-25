-- +goose Up
CREATE TABLE email_verification_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_evt_token_hash ON email_verification_tokens (token_hash);
CREATE INDEX idx_evt_user_id    ON email_verification_tokens (user_id);

-- +goose Down
DROP TABLE IF EXISTS email_verification_tokens;
