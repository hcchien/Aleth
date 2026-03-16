-- +goose Up
CREATE TABLE user_credentials (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type            TEXT NOT NULL,          -- 'password' | 'google' | 'facebook' | 'passkey'
    credential_id   TEXT,                   -- passkey credential id / oauth sub
    credential_data BYTEA,                  -- hashed password or passkey public key
    sign_count      BIGINT,                 -- passkey replay protection
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_user_credentials_user_id ON user_credentials (user_id);
CREATE UNIQUE INDEX idx_user_credentials_type_credential ON user_credentials (type, credential_id)
    WHERE credential_id IS NOT NULL;

CREATE TABLE refresh_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      TEXT UNIQUE NOT NULL,   -- sha256(token)
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens (user_id);
CREATE INDEX idx_refresh_tokens_expires ON refresh_tokens (expires_at) WHERE revoked_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS user_credentials;
