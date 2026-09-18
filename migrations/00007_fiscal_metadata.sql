-- +goose Up
ALTER TABLE companies ADD COLUMN fiscal_year_end text;
-- +goose Down
ALTER TABLE companies DROP COLUMN fiscal_year_end;
