-- vc_type_registry: canonical list of VC types that the platform and users can define.
-- issuer is the namespace owner — "platform" for built-ins, or a user's username for custom types.
-- Board policy requireVcs entries must reference a (vc_type, issuer) pair present in this table.

CREATE TABLE vc_type_registry (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    vc_type     TEXT        NOT NULL,
    issuer      TEXT        NOT NULL,
    label       TEXT        NOT NULL,
    description TEXT,
    created_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
    enabled     BOOLEAN     NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (vc_type, issuer)
);

-- Platform-issued built-ins (no created_by)
INSERT INTO vc_type_registry (vc_type, issuer, label, description) VALUES
    ('verified_citizen',  'platform', 'Verified Citizen',  'Government-issued ID verified by the platform'),
    ('basic_id',          'platform', 'Basic ID',           'Platform identity verification (email + phone)'),
    ('press_credential',  'platform', 'Press Credential',   'Verified journalist credential'),
    ('researcher',        'platform', 'Researcher',         'Academic or research institution affiliation');
