-- +goose Up
ALTER TABLE quarterly_financials ADD COLUMN revenue_source TEXT, ADD COLUMN eps_source TEXT;
UPDATE quarterly_financials SET revenue_source=CASE WHEN revenue IS NOT NULL THEN source END,
 eps_source=CASE WHEN diluted_eps IS NOT NULL THEN source END;
ALTER TABLE historical_prices ADD COLUMN split_ratio NUMERIC CHECK(split_ratio > 0);
-- +goose Down
ALTER TABLE historical_prices DROP COLUMN split_ratio;
ALTER TABLE quarterly_financials DROP COLUMN revenue_source, DROP COLUMN eps_source;
