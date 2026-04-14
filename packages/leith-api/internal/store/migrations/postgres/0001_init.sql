CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  did TEXT NOT NULL UNIQUE,
  oauth_id TEXT NOT NULL DEFAULT '',
  public_key BYTEA,
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
