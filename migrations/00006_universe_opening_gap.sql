-- +goose Up
ALTER TABLE companies ADD COLUMN security_type text, ADD COLUMN exchange_code text,
 ADD COLUMN universe_eligible boolean NOT NULL DEFAULT false,
 ADD COLUMN universe_reason text NOT NULL DEFAULT 'not_evaluated',
 ADD COLUMN universe_checked_at timestamptz;
CREATE INDEX companies_universe ON companies(symbol) WHERE universe_eligible;
ALTER TABLE earnings_reactions ADD COLUMN opening_gap_pct numeric;

-- +goose Down
ALTER TABLE earnings_reactions DROP COLUMN opening_gap_pct;
DROP INDEX companies_universe;
ALTER TABLE companies DROP COLUMN security_type, DROP COLUMN exchange_code,
 DROP COLUMN universe_eligible, DROP COLUMN universe_reason, DROP COLUMN universe_checked_at;
