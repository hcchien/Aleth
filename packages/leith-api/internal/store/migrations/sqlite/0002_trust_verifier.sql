CREATE TABLE IF NOT EXISTS credentials (
  id TEXT PRIMARY KEY,
  subject_did TEXT NOT NULL,
  issuer_did TEXT NOT NULL,
  credential_type TEXT NOT NULL,
  claims_json TEXT NOT NULL DEFAULT '{}',
  proof TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  issued_at DATETIME NOT NULL,
  expires_at DATETIME NULL,
  revoked_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS trust_assessments (
  id TEXT PRIMARY KEY,
  subject_did TEXT NOT NULL,
  tier INTEGER NOT NULL,
  score REAL NOT NULL DEFAULT 1.0,
  source TEXT NOT NULL,
  evidence_refs TEXT NOT NULL DEFAULT '[]',
  issued_by_did TEXT NOT NULL DEFAULT '',
  effective_at DATETIME NOT NULL,
  expires_at DATETIME NULL,
  revoked_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS verifiers (
  id TEXT PRIMARY KEY,
  verifier_did TEXT NOT NULL UNIQUE,
  verifier_type TEXT NOT NULL,
  scope TEXT NOT NULL,
  authority_level INTEGER NOT NULL,
  status TEXT NOT NULL,
  appointed_by_did TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  expires_at DATETIME NULL,
  revoked_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS verification_cases (
  id TEXT PRIMARY KEY,
  subject_did TEXT NOT NULL,
  requested_tier INTEGER NOT NULL,
  credential_type TEXT NOT NULL,
  evidence_json TEXT NOT NULL DEFAULT '{}',
  status TEXT NOT NULL,
  assigned_verifier_did TEXT NULL,
  decision TEXT NOT NULL DEFAULT '',
  decision_reason TEXT NOT NULL DEFAULT '',
  created_at DATETIME NOT NULL,
  decided_at DATETIME NULL
);

CREATE TABLE IF NOT EXISTS verifier_decisions (
  id TEXT PRIMARY KEY,
  case_id TEXT NOT NULL,
  verifier_did TEXT NOT NULL,
  decision TEXT NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  issued_credential_id TEXT NULL,
  issued_assessment_id TEXT NULL,
  created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS trust_audit_logs (
  id TEXT PRIMARY KEY,
  actor_did TEXT NOT NULL,
  action_type TEXT NOT NULL,
  target_did TEXT NOT NULL DEFAULT '',
  target_resource_id TEXT NOT NULL DEFAULT '',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at DATETIME NOT NULL
);
