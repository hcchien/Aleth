-- Board-level VC policy for domain-gated participation.
--
-- require_vcs is a JSONB array of VC requirements that a user must satisfy
-- to write content (articles, comments) on this board.  An empty array
-- means "no VC required" — trust level alone gates access.
--
-- Format:
--   [{"type":"NationalID","issuer":"GOV_TW"},
--    {"type":"Journalist","issuer":"PRESS_ASSOC_TW"}]
--
-- Semantics: ALL listed VCs must be present (AND logic).  OR logic can be
-- expressed by the application layer if needed in the future.
--
-- Also expose min_trust_level and min_comment_trust via a rename so the API
-- layer can manage them uniformly with the new VC policy fields.

ALTER TABLE boards
    ADD COLUMN require_vcs         JSONB    NOT NULL DEFAULT '[]',
    ADD COLUMN require_comment_vcs JSONB    NOT NULL DEFAULT '[]';

COMMENT ON COLUMN boards.require_vcs IS
    'JSON array of {type,issuer} VC requirements for writing articles on this board.';

COMMENT ON COLUMN boards.require_comment_vcs IS
    'JSON array of {type,issuer} VC requirements for commenting on articles in this board.';
