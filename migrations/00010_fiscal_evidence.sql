-- +goose Up
CREATE TABLE fiscal_periods (
 company_id bigint NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
 period_end date NOT NULL,
 fiscal_year integer NOT NULL,
 fiscal_quarter integer NOT NULL CHECK(fiscal_quarter BETWEEN 0 AND 4),
 filed date NOT NULL,
 accession text NOT NULL,
 form text NOT NULL,
 mapping_source text NOT NULL,
 ambiguous boolean NOT NULL DEFAULT false,
 PRIMARY KEY(company_id,period_end)
);
ALTER TABLE earnings_events ADD COLUMN fiscal_mapping_reason text;
ALTER TABLE earnings_events ADD COLUMN revenue_actual_source text;
ALTER TABLE earnings_events ADD COLUMN revenue_estimate_source text;
-- Earlier versions used percentage points. Revenue alone now uses fractions.
UPDATE earnings_events SET revenue_surprise_pct=revenue_surprise_pct/100 WHERE revenue_surprise_pct IS NOT NULL;
COMMENT ON COLUMN earnings_events.revenue_surprise_pct IS 'Fraction: (actual-estimate)/abs(estimate); EPS retains percentage points';

-- +goose Down
UPDATE earnings_events SET revenue_surprise_pct=revenue_surprise_pct*100 WHERE revenue_surprise_pct IS NOT NULL;
ALTER TABLE earnings_events DROP COLUMN fiscal_mapping_reason, DROP COLUMN revenue_actual_source, DROP COLUMN revenue_estimate_source;
DROP TABLE fiscal_periods;
