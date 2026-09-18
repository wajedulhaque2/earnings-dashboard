package repository

import (
	"context"
	"earnings-dashboard/internal/models"
	"encoding/json"
	"errors"
)

// ApplyUniverse atomically publishes a complete snapshot. Partial provider
// failures never empty the previously selected universe.
func (s *Store) ApplyUniverse(ctx context.Context, companies []models.Company) error {
	if len(companies) == 0 {
		return errors.New("empty universe snapshot")
	}
	data, err := json.Marshal(companies)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `WITH input AS (SELECT * FROM jsonb_to_recordset($1::jsonb) AS x("Symbol" text,"Name" text,"ExchangeCode" text,"SecurityType" text,"Currency" text,"MarketCap" numeric,"Sector" text,"Industry" text,"UniverseEligible" boolean,"UniverseReason" text)),
 cleared AS (UPDATE companies SET universe_eligible=false,universe_reason='not_in_latest_screen',universe_checked_at=now() WHERE symbol NOT IN(SELECT "Symbol" FROM input))
 INSERT INTO companies(symbol,name,exchange,exchange_code,security_type,currency,market_cap,sector,industry,universe_eligible,universe_reason,universe_checked_at)
 SELECT "Symbol","Name","ExchangeCode","ExchangeCode","SecurityType","Currency","MarketCap","Sector","Industry","UniverseEligible","UniverseReason",now() FROM input
 ON CONFLICT(symbol) DO UPDATE SET name=COALESCE(EXCLUDED.name,companies.name),exchange_code=EXCLUDED.exchange_code,security_type=EXCLUDED.security_type,currency=EXCLUDED.currency,market_cap=EXCLUDED.market_cap,sector=COALESCE(EXCLUDED.sector,companies.sector),industry=COALESCE(EXCLUDED.industry,companies.industry),universe_eligible=EXCLUDED.universe_eligible,universe_reason=EXCLUDED.universe_reason,universe_checked_at=now()`, string(data))
	return err
}

func (s *Store) QueueCalendar(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `INSERT INTO sync_requests(symbol)
 SELECT DISTINCT c.symbol FROM companies c JOIN earnings_events e ON e.company_id=c.id
 WHERE c.universe_eligible AND e.report_date BETWEEN (now() AT TIME ZONE 'America/New_York')::date-7 AND (now() AT TIME ZONE 'America/New_York')::date+14
 AND NOT EXISTS(SELECT 1 FROM provider_sync_state p WHERE p.provider='yahoo' AND p.resource='all:'||c.symbol AND p.latest_attempt_at>now()-interval '6 hours')
 ON CONFLICT(symbol) DO NOTHING`)
	return err
}

func (s *Store) UniverseDue(ctx context.Context) (bool, error) {
	var due bool
	err := s.db.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM provider_sync_state WHERE provider='yahoo' AND resource='universe:' AND latest_attempt_at>now()-interval '24 hours')`).Scan(&due)
	return due, err
}
