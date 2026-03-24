-- +goose Up
-- Allow series to belong to either a board OR a fan page.
ALTER TABLE series ALTER COLUMN board_id DROP NOT NULL;
ALTER TABLE series ADD COLUMN page_id UUID REFERENCES fan_pages(id) ON DELETE CASCADE;
CREATE INDEX idx_series_page_id ON series (page_id) WHERE page_id IS NOT NULL;
ALTER TABLE series ADD CONSTRAINT series_has_owner CHECK (
    (board_id IS NOT NULL AND page_id IS NULL) OR
    (board_id IS NULL AND page_id IS NOT NULL)
);

-- +goose Down
ALTER TABLE series DROP CONSTRAINT series_has_owner;
DROP INDEX idx_series_page_id;
ALTER TABLE series DROP COLUMN page_id;
ALTER TABLE series ALTER COLUMN board_id SET NOT NULL;
