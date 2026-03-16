-- +goose Up
ALTER TABLE refresh_tokens
    ADD COLUMN session_trust_level SMALLINT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE refresh_tokens
    DROP COLUMN session_trust_level;
