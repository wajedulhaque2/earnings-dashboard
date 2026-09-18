-- +goose Up
-- Preserve raw observations. Suppress a Yahoo month-end duplicate only when
-- BOTH reported values and currency exactly agree with a nearby SEC period.
CREATE VIEW canonical_quarterly_financials AS
SELECT y.* FROM quarterly_financials y
WHERE y.source<>'yahoo' OR NOT EXISTS (
 SELECT 1 FROM quarterly_financials s
 WHERE s.company_id=y.company_id AND s.source='sec' AND s.revenue_source='sec' AND s.eps_source='sec' AND s.currency=y.currency
 AND s.period_end<>y.period_end AND abs(s.period_end-y.period_end)<=7
 AND date_trunc('month',s.period_end)=date_trunc('month',y.period_end)
 AND s.revenue=y.revenue AND s.diluted_eps=y.diluted_eps
);
-- +goose Down
DROP VIEW canonical_quarterly_financials;
