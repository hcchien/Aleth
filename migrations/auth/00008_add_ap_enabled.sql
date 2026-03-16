-- Add ap_enabled flag so users can opt out of ActivityPub federation.
-- Defaults to true (all existing users are discoverable by default).

ALTER TABLE users ADD COLUMN ap_enabled BOOLEAN NOT NULL DEFAULT true;

---- create above / drop below ----

ALTER TABLE users DROP COLUMN IF EXISTS ap_enabled;
