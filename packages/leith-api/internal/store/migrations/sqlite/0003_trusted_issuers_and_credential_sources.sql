ALTER TABLE credentials ADD COLUMN external_issuer_did TEXT NULL;
ALTER TABLE credentials ADD COLUMN issuance_source TEXT NOT NULL DEFAULT 'internal_verifier_issued';

CREATE TABLE IF NOT EXISTS trusted_issuers (
  id TEXT PRIMARY KEY,
  issuer_did TEXT NOT NULL UNIQUE,
  issuer_name TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  scopes TEXT NOT NULL DEFAULT '[]',
  credential_types TEXT NOT NULL DEFAULT '[]',
  max_trust_tier_issued INTEGER NOT NULL,
  appointed_by_did TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  expires_at DATETIME NULL,
  revoked_at DATETIME NULL
);

ALTER TABLE verifier_decisions ADD COLUMN credential_issuance_source TEXT NOT NULL DEFAULT 'internal_verifier_issued';
ALTER TABLE verifier_decisions ADD COLUMN external_issuer_did TEXT NULL;
