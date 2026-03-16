-- Verifiable Credentials (VCs) attached to user accounts.
--
-- Phase 1 (custodial): Platform admins or OAuth-verified flows write rows
-- directly after checking the external credential.  The raw VC payload is
-- NOT stored here — only the extracted, verified claims we care about.
--
-- Phase 2 (ZKP): The 'attributes' column stays the same; the issuer call
-- switches to verifying a ZK proof so we never see the full credential.
--
-- Uniqueness: one row per (user, vc_type, issuer) — re-verifying the same
-- credential type from the same issuer is an upsert, not a new row.

CREATE TABLE user_vcs (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- What kind of credential and who vouches for it.
    -- Examples:  vc_type='NationalID'   issuer='GOV_TW'
    --            vc_type='Journalist'   issuer='PRESS_ASSOC_TW'
    --            vc_type='MedicalLicense' issuer='MOHW_TW'
    vc_type     TEXT        NOT NULL,
    issuer      TEXT        NOT NULL,

    -- Key–value claims extracted from the credential.
    -- Examples: {"country":"TW"}, {"age":30,"license_no":"A123"}
    -- Intentionally NOT storing PII beyond what the policy engine needs.
    attributes  JSONB       NOT NULL DEFAULT '{}',

    verified_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ,         -- NULL = does not expire
    revoked_at  TIMESTAMPTZ,         -- NULL = still valid

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Fast lookup of all VCs for a given user.
CREATE INDEX ON user_vcs (user_id);

-- Prevent duplicate VC entries; re-verification upserts this row.
CREATE UNIQUE INDEX ON user_vcs (user_id, vc_type, issuer);
