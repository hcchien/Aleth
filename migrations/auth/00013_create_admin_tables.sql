-- +goose Up

-- Admin accounts are completely separate from platform users.
-- Stored in the auth DB so queries can join with the users table directly.
CREATE TABLE admin_users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT        NOT NULL UNIQUE,
    email         TEXT        NOT NULL UNIQUE,
    password_hash BYTEA       NOT NULL,
    -- 'moderator'  → resolve reports, delete posts
    -- 'admin'      → all above + manage trust levels, suspend users
    -- 'superadmin' → all above + manage other admin accounts
    role          TEXT        NOT NULL DEFAULT 'moderator',
    is_active     BOOLEAN     NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ
);

-- Immutable audit trail of every admin action.
CREATE TABLE audit_log (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id    UUID        NOT NULL REFERENCES admin_users(id),
    action      TEXT        NOT NULL,
    -- 'delete_post' | 'resolve_report' | 'dismiss_report'
    -- 'set_trust_level' | 'suspend_user' | 'unsuspend_user' | 'delete_user'
    target_type TEXT        NOT NULL,  -- 'post' | 'user' | 'report'
    target_id   UUID        NOT NULL,
    note        TEXT,
    metadata    JSONB,                 -- e.g. {"before": 2, "after": 3} for trust level changes
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_admin   ON audit_log (admin_id, created_at DESC);
CREATE INDEX idx_audit_target  ON audit_log (target_type, target_id);
CREATE INDEX idx_audit_created ON audit_log (created_at DESC);

-- Performance index for admin user listing (most recent first).
CREATE INDEX idx_users_created_desc ON users (created_at DESC) WHERE deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_created_desc;
DROP INDEX IF EXISTS idx_audit_created;
DROP INDEX IF EXISTS idx_audit_target;
DROP INDEX IF EXISTS idx_audit_admin;
DROP TABLE IF EXISTS audit_log;
DROP TABLE IF EXISTS admin_users;
