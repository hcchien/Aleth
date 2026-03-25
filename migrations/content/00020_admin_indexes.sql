-- +goose Up

-- Composite partial index for the admin reports queue.
-- Covers the grouped-by-post view: for each post, fetch its unresolved reports
-- ordered by recency, without a full-table scan.
CREATE INDEX idx_reports_post_unresolved
    ON reports (post_id, created_at DESC)
    WHERE resolved_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_reports_post_unresolved;
