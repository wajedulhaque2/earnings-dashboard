-- +goose Up
ALTER TABLE companies ADD COLUMN latest_price NUMERIC, ADD COLUMN quote_time TIMESTAMPTZ,
 ADD COLUMN timezone TEXT, ADD COLUMN cik TEXT;
ALTER TABLE earnings_events ADD COLUMN period_end DATE;
-- Fiscal labels cannot be safely inferred from a calendar period end.
ALTER TABLE quarterly_financials ALTER COLUMN fiscal_year DROP NOT NULL,
 ALTER COLUMN fiscal_quarter DROP NOT NULL, ADD COLUMN currency TEXT;
ALTER TABLE quarterly_financials ADD CONSTRAINT financial_period_unique UNIQUE(company_id, period_end);
ALTER TABLE earnings_reactions ADD COLUMN event_date DATE;
CREATE TABLE historical_prices (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 trading_date DATE NOT NULL, open NUMERIC, high NUMERIC, low NUMERIC,
 close NUMERIC CHECK(close > 0), adjusted_close NUMERIC CHECK(adjusted_close > 0), volume BIGINT CHECK(volume >= 0),
 source TEXT NOT NULL CHECK(source IN ('yahoo','massive')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE(company_id,trading_date,source)
);
CREATE TABLE intraday_snapshots (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 timestamp TIMESTAMPTZ NOT NULL, session TEXT NOT NULL CHECK(session IN ('PRE','REGULAR','POST')),
 price NUMERIC NOT NULL CHECK(price > 0), source TEXT NOT NULL CHECK(source IN ('yahoo','massive')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(company_id,timestamp,source)
);
CREATE TABLE filings (
 company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 accession TEXT NOT NULL, form TEXT NOT NULL, filed DATE NOT NULL, report_date DATE,
 document TEXT NOT NULL, PRIMARY KEY(company_id,accession)
);
CREATE TABLE sync_requests (
 symbol TEXT PRIMARY KEY, requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 attempts INTEGER NOT NULL DEFAULT 0, next_attempt TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX companies_name_idx ON companies(lower(name));

-- +goose Down
DROP INDEX companies_name_idx;
DROP TABLE sync_requests;
DROP TABLE filings;
DROP TABLE intraday_snapshots;
DROP TABLE historical_prices;
ALTER TABLE earnings_reactions DROP COLUMN event_date;
ALTER TABLE quarterly_financials DROP CONSTRAINT financial_period_unique, DROP COLUMN currency;
-- Do not invent fiscal labels during rollback; retain their nullable semantics.
ALTER TABLE earnings_events DROP COLUMN period_end;
ALTER TABLE companies DROP COLUMN latest_price, DROP COLUMN quote_time, DROP COLUMN timezone, DROP COLUMN cik;
