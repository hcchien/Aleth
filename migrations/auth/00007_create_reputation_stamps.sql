-- reputation_stamps: one row per (user, provider) stamp.
-- Scores are summed to determine L2 eligibility.
-- Providers: 'phone', 'instagram', 'facebook', 'twitter', 'linkedin'

CREATE TABLE reputation_stamps (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id         UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider        TEXT        NOT NULL,   -- 'phone' | 'instagram' | 'facebook' | 'twitter' | 'linkedin'
    provider_user_id TEXT,                  -- external account ID or phone number
    score           SMALLINT    NOT NULL DEFAULT 0,
    metadata        JSONB,                  -- account_age_days, follower_count, etc.
    verified_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,           -- NULL = never; set to NOW()+30d for OAuth stamps
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, provider)             -- one stamp per provider per user
);

CREATE INDEX idx_reputation_stamps_user_id ON reputation_stamps (user_id);

-- ── phone_otps: temporary OTP codes for phone verification ──────────────────
-- Keyed by (user_id, phone); expires quickly.

CREATE TABLE phone_otps (
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    phone       TEXT        NOT NULL,
    code        TEXT        NOT NULL,
    attempts    SMALLINT    NOT NULL DEFAULT 0,
    expires_at  TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (user_id, phone)
);

---- create above / drop below ----

DROP TABLE IF EXISTS phone_otps;
DROP TABLE IF EXISTS reputation_stamps;
