CREATE TABLE IF NOT EXISTS wallet_presentation_requests (
  id TEXT PRIMARY KEY,
  subject_did TEXT NOT NULL,
  verifier_did TEXT NOT NULL,
  requested_tier INTEGER NOT NULL,
  credential_type TEXT NOT NULL,
  purpose TEXT NOT NULL DEFAULT '',
  allowed_issuer_dids JSONB NOT NULL DEFAULT '[]'::jsonb,
  challenge TEXT NOT NULL,
  request_uri TEXT NOT NULL,
  qr_payload TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NULL,
  completed_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS wallet_presentation_verifications (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL REFERENCES wallet_presentation_requests(id),
  subject_did TEXT NOT NULL,
  issuer_did TEXT NOT NULL,
  credential_type TEXT NOT NULL,
  presentation_format TEXT NOT NULL,
  claims_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  proof TEXT NOT NULL DEFAULT '',
  audience TEXT NOT NULL DEFAULT '',
  nonce TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  trusted_issuer_did TEXT NULL,
  notes TEXT NOT NULL DEFAULT '',
  issued_credential_id TEXT NULL,
  issued_assessment_id TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  verified_at TIMESTAMPTZ NULL
);
