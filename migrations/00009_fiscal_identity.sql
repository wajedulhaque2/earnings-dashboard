-- +goose Up
-- Period end is the observation identity. Provider fiscal labels can conflict
-- across historical restatements and must never prevent numeric ingestion.
ALTER TABLE quarterly_financials DROP CONSTRAINT quarterly_financials_company_id_fiscal_year_fiscal_quarter_key;
ALTER TABLE earnings_events ADD COLUMN fiscal_label_source text;
UPDATE earnings_events SET fiscal_label_source=CASE WHEN period_end IS NOT NULL THEN 'sec_same_day' ELSE 'yahoo_event' END WHERE fiscal_year IS NOT NULL;

-- +goose Down
ALTER TABLE earnings_events DROP COLUMN fiscal_label_source;
-- Keep labels nullable on rollback rather than discarding conflicting observations.
