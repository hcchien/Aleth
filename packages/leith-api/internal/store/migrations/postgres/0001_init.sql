CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  did TEXT NOT NULL UNIQUE,
  oauth_id TEXT NOT NULL DEFAULT '',
  public_key BYTEA,
  authn_user_id BYTEA,
  passkey_credentials BYTEA NOT NULL DEFAULT ''::bytea,
  trust_tier INTEGER NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS posts (
  id BIGSERIAL PRIMARY KEY,
  body TEXT NOT NULL,
  media_hashes TEXT NOT NULL,
  parent_id BIGINT NULL,
  timestamp BIGINT NOT NULL,
  author_did TEXT NOT NULL,
  signature TEXT NOT NULL,
  visibility_score DOUBLE PRECISION NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS content_items (
  id TEXT PRIMARY KEY,
  author_did TEXT NOT NULL,
  title TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL,
  mode TEXT NOT NULL,
  status TEXT NOT NULL,
  visibility TEXT NOT NULL,
  trust_tier INTEGER NOT NULL,
  participation_policy TEXT NOT NULL DEFAULT '',
  discussion_shape TEXT NOT NULL DEFAULT '',
  source_content_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  published_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS discussion_nodes (
  id TEXT PRIMARY KEY,
  discussion_id TEXT NOT NULL,
  parent_node_id TEXT NULL,
  author_did TEXT NOT NULL,
  node_type TEXT NOT NULL,
  stance TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS content_relations (
  id TEXT PRIMARY KEY,
  from_content_id TEXT NOT NULL,
  to_content_id TEXT NOT NULL,
  relation_type TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS projections (
  id TEXT PRIMARY KEY,
  source_idea_id TEXT NOT NULL,
  target_discussion_id TEXT NOT NULL,
  projected_excerpt TEXT NOT NULL,
  participation_policy TEXT NOT NULL,
  ownership_transfer_acknowledged BOOLEAN NOT NULL,
  created_by_did TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS transformation_jobs (
  id TEXT PRIMARY KEY,
  requested_by_did TEXT NOT NULL,
  source_content_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  target_mode TEXT NOT NULL,
  provider_type TEXT NOT NULL,
  prompt_profile TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  output_title TEXT NOT NULL DEFAULT '',
  output_body TEXT NOT NULL DEFAULT '',
  published_content_id TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS discussion_forks (
  id TEXT PRIMARY KEY,
  source_discussion_id TEXT NOT NULL,
  fork_discussion_id TEXT NOT NULL,
  created_by_did TEXT NOT NULL,
  reason TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS moderation_actions (
  id TEXT PRIMARY KEY,
  target_content_id TEXT NOT NULL,
  action_type TEXT NOT NULL,
  reason TEXT NOT NULL,
  initiated_by_did TEXT NOT NULL,
  required_tier INTEGER NOT NULL,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS credentials (
  id TEXT PRIMARY KEY,
  subject_did TEXT NOT NULL,
  issuer_did TEXT NOT NULL,
  external_issuer_did TEXT NULL,
  credential_type TEXT NOT NULL,
  claims_json TEXT NOT NULL DEFAULT '{}',
  proof TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL,
  issuance_source TEXT NOT NULL DEFAULT 'internal_verifier_issued',
  issued_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NULL,
  revoked_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS trust_assessments (
  id TEXT PRIMARY KEY,
  subject_did TEXT NOT NULL,
  tier INTEGER NOT NULL,
  score DOUBLE PRECISION NOT NULL DEFAULT 1.0,
  source TEXT NOT NULL,
  evidence_refs JSONB NOT NULL DEFAULT '[]'::jsonb,
  issued_by_did TEXT NOT NULL DEFAULT '',
  effective_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NULL,
  revoked_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS verifiers (
  id TEXT PRIMARY KEY,
  verifier_did TEXT NOT NULL UNIQUE,
  verifier_type TEXT NOT NULL,
  scope TEXT NOT NULL,
  authority_level INTEGER NOT NULL,
  status TEXT NOT NULL,
  appointed_by_did TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NULL,
  revoked_at TIMESTAMPTZ NULL
);

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
  created_at TIMESTAMPTZ NOT NULL,
  decided_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS verifier_decisions (
  id TEXT PRIMARY KEY,
  case_id TEXT NOT NULL,
  verifier_did TEXT NOT NULL,
  decision TEXT NOT NULL,
  reason TEXT NOT NULL DEFAULT '',
  credential_issuance_source TEXT NOT NULL DEFAULT 'internal_verifier_issued',
  external_issuer_did TEXT NULL,
  issued_credential_id TEXT NULL,
  issued_assessment_id TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS trust_audit_logs (
  id TEXT PRIMARY KEY,
  actor_did TEXT NOT NULL,
  action_type TEXT NOT NULL,
  target_did TEXT NOT NULL DEFAULT '',
  target_resource_id TEXT NOT NULL DEFAULT '',
  metadata_json TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL
);
