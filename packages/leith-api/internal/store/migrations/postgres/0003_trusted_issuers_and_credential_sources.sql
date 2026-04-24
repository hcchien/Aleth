ALTER TABLE credentials ADD COLUMN IF NOT EXISTS external_issuer_did TEXT NULL;
ALTER TABLE credentials ADD COLUMN IF NOT EXISTS issuance_source TEXT NOT NULL DEFAULT 'internal_verifier_issued';

CREATE TABLE IF NOT EXISTS trusted_issuers (
  id TEXT PRIMARY KEY,
  issuer_did TEXT NOT NULL UNIQUE,
  issuer_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
  credential_types JSONB NOT NULL DEFAULT '[]'::jsonb,
  max_trust_tier_issued INTEGER NOT NULL,
  appointed_by_did TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NULL,
  revoked_at TIMESTAMPTZ NULL
);

ALTER TABLE verifier_decisions ADD COLUMN IF NOT EXISTS credential_issuance_source TEXT NOT NULL DEFAULT 'internal_verifier_issued';
ALTER TABLE verifier_decisions ADD COLUMN IF NOT EXISTS external_issuer_did TEXT NULL;
