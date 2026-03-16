-- actor_keys: one RSA-2048 key pair per Aleth user.
-- The private key is AES-256-GCM encrypted with the platform master secret.
-- Keys are generated lazily on first actor-endpoint access.

CREATE TABLE actor_keys (
    id              UUID        NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
    user_id         UUID        UNIQUE NOT NULL,   -- logical FK → auth.users(id)
    username        TEXT        NOT NULL,
    public_key_pem  TEXT        NOT NULL,          -- PKIX / "BEGIN PUBLIC KEY"
    private_key_enc BYTEA       NOT NULL,          -- AES-256-GCM(PKCS#8 PEM, platform_key_secret)
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_actor_keys_username ON actor_keys (username);

---- create above / drop below ----

DROP TABLE IF EXISTS actor_keys;
