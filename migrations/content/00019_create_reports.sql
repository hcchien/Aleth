-- +goose Up
CREATE TYPE report_reason AS ENUM (
    'spam',
    'harassment',
    'hate_speech',
    'misinformation',
    'illegal_content',
    'other'
);

CREATE TABLE reports (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    reporter_id UUID        NOT NULL,  -- user who filed the report (FK to auth.users)
    post_id     UUID        NOT NULL REFERENCES posts (id) ON DELETE CASCADE,
    reason      report_reason NOT NULL,
    note        TEXT,                  -- optional extra detail from the reporter
    resolved_at TIMESTAMPTZ,          -- NULL = open; set by a moderator
    resolved_by UUID,                 -- moderator's user ID
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reports_post_id ON reports (post_id);
CREATE INDEX idx_reports_reporter ON reports (reporter_id);
CREATE INDEX idx_reports_open ON reports (created_at) WHERE resolved_at IS NULL;

-- Prevent the same user reporting the same post twice for the same reason.
CREATE UNIQUE INDEX idx_reports_unique ON reports (reporter_id, post_id, reason);

-- +goose Down
DROP TABLE IF EXISTS reports;
DROP TYPE IF EXISTS report_reason;
