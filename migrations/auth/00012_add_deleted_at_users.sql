-- +goose Up
ALTER TABLE users ADD COLUMN deleted_at TIMESTAMPTZ;

-- Deleted users must not be discoverable by username or email.
CREATE UNIQUE INDEX idx_users_username_active ON users (username) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_users_email_active    ON users (email)    WHERE deleted_at IS NULL AND email IS NOT NULL;

-- Drop the original unique constraints that don't filter on deleted_at.
-- (The partial indexes above enforce uniqueness only for active accounts.)
ALTER TABLE users DROP CONSTRAINT users_username_key;
ALTER TABLE users DROP CONSTRAINT users_email_key;

-- +goose Down
ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);
ALTER TABLE users ADD CONSTRAINT users_email_key    UNIQUE (email);

DROP INDEX IF EXISTS idx_users_username_active;
DROP INDEX IF EXISTS idx_users_email_active;

ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
