-- +goose Up
CREATE TABLE series (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id    UUID NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_series_board_id ON series (board_id);

ALTER TABLE articles ADD COLUMN series_id UUID REFERENCES series(id) ON DELETE SET NULL;

CREATE INDEX idx_articles_series_id ON articles (series_id) WHERE series_id IS NOT NULL;

-- +goose Down
ALTER TABLE articles DROP COLUMN series_id;
DROP TABLE series;
