-- +goose Up
CREATE TABLE companies (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 symbol TEXT NOT NULL UNIQUE CHECK (symbol ~ '^[A-Z0-9^][A-Z0-9.^=/_-]{0,31}$'),
 name TEXT, exchange TEXT, country TEXT, sector TEXT, industry TEXT,
 description TEXT, logo_url TEXT, market_cap NUMERIC(24,0) CHECK (market_cap >= 0),
 currency TEXT, active BOOLEAN NOT NULL DEFAULT TRUE,
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE earnings_events (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 fiscal_year INTEGER, fiscal_quarter SMALLINT CHECK (fiscal_quarter BETWEEN 1 AND 4),
 report_date DATE NOT NULL, report_time TIMESTAMPTZ,
 session TEXT NOT NULL DEFAULT 'UNKNOWN' CHECK (session IN ('BMO','AMC','DURING_MARKET','UNKNOWN')),
 eps_estimate NUMERIC, eps_actual NUMERIC, eps_surprise NUMERIC, eps_surprise_pct NUMERIC,
 revenue_estimate NUMERIC, revenue_actual NUMERIC, revenue_surprise NUMERIC, revenue_surprise_pct NUMERIC,
 source TEXT NOT NULL CHECK (source IN ('yahoo','sec','massive','alphavantage')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE (company_id, report_date)
);
CREATE INDEX earnings_events_calendar_idx ON earnings_events(report_date, session);
CREATE TABLE quarterly_financials (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 fiscal_year INTEGER NOT NULL, fiscal_quarter SMALLINT NOT NULL CHECK (fiscal_quarter BETWEEN 1 AND 4),
 period_end DATE NOT NULL, revenue NUMERIC, diluted_eps NUMERIC,
 source TEXT NOT NULL CHECK (source IN ('yahoo','sec','massive','alphavantage')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE (company_id, fiscal_year, fiscal_quarter)
);
CREATE TABLE earnings_reactions (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 earnings_event_id BIGINT NOT NULL UNIQUE REFERENCES earnings_events(id) ON DELETE CASCADE,
 previous_close NUMERIC CHECK (previous_close > 0), premarket_price NUMERIC CHECK (premarket_price > 0),
 premarket_return_pct NUMERIC, event_open NUMERIC CHECK (event_open > 0), event_close NUMERIC CHECK (event_close > 0),
 event_day_return_pct NUMERIC, return_1d NUMERIC, return_2d NUMERIC, return_1w NUMERIC,
 return_2w NUMERIC, return_1m NUMERIC, return_3m NUMERIC,
 calculated_at TIMESTAMPTZ NOT NULL DEFAULT now(), methodology_version TEXT NOT NULL
);
CREATE TABLE watchlists (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 name TEXT NOT NULL UNIQUE, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO watchlists (name) VALUES ('Default');
CREATE TABLE watchlist_companies (
 watchlist_id BIGINT NOT NULL REFERENCES watchlists(id) ON DELETE CASCADE,
 company_id BIGINT NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 PRIMARY KEY (watchlist_id, company_id)
);
CREATE INDEX watchlist_companies_company_idx ON watchlist_companies(company_id);
CREATE TABLE provider_sync_state (
 provider TEXT NOT NULL CHECK (provider IN ('yahoo','sec','massive','alphavantage')),
 resource TEXT NOT NULL, last_sync TIMESTAMPTZ,
 status TEXT NOT NULL CHECK (status IN ('pending','running','success','error')),
 error TEXT, PRIMARY KEY (provider, resource)
);
COMMENT ON COLUMN provider_sync_state.error IS 'Sanitized error category only; never credentials or raw provider responses.';
COMMENT ON COLUMN earnings_events.report_date IS 'US earnings dates interpreted in America/New_York; not necessarily the reaction session.';
COMMENT ON TABLE earnings_reactions IS 'Return units and trading-session methodology must be defined by methodology_version before writing analytics.';

-- +goose Down
DROP TABLE provider_sync_state;
DROP TABLE watchlist_companies;
DROP TABLE watchlists;
DROP TABLE earnings_reactions;
DROP TABLE quarterly_financials;
DROP TABLE earnings_events;
DROP TABLE companies;
