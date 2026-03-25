-- +goose Up
ALTER TABLE posts ADD COLUMN image_urls TEXT[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE posts DROP COLUMN image_urls;
