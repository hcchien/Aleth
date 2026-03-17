-- +goose Up
ALTER TABLE fan_pages ADD COLUMN post_count INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE fan_pages DROP COLUMN post_count;
