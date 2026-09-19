package repository

import "context"

func (s *Store) USDRevenueHistory(ctx context.Context, id int64) (bool, error) {
	var usd bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM quarterly_financials WHERE company_id=$1 AND revenue IS NOT NULL AND currency='USD') AND NOT EXISTS(SELECT 1 FROM quarterly_financials WHERE company_id=$1 AND revenue IS NOT NULL AND currency IS DISTINCT FROM 'USD')`, id).Scan(&usd)
	return usd, err
}

func (s *Store) RevenueCache(ctx context.Context, symbol, operation string) ([]byte, error) {
	var b []byte
	err := s.db.QueryRow(ctx, `SELECT COALESCE((SELECT payload FROM revenue_provider_cache WHERE symbol=$1 AND operation=$2 AND fetched_at>now()-CASE WHEN payload->>'unavailable'='true' THEN interval '1 day' ELSE interval '7 days' END),'null'::jsonb)`, symbol, operation).Scan(&b)
	if string(b) == "null" {
		b = nil
	}
	return b, err
}
func (s *Store) SaveRevenueCache(ctx context.Context, symbol, operation string, b []byte) error {
	_, err := s.db.Exec(ctx, `INSERT INTO revenue_provider_cache(symbol,operation,payload) VALUES($1,$2,$3::jsonb) ON CONFLICT(symbol,operation) DO UPDATE SET payload=EXCLUDED.payload,fetched_at=now()`, symbol, operation, b)
	return err
}
func (s *Store) ReserveRevenueRequest(ctx context.Context) (bool, error) {
	result, err := s.db.Exec(ctx, `INSERT INTO revenue_provider_budget(provider,request_times,last_request) VALUES('alphavantage',ARRAY[now()],now()) ON CONFLICT(provider) DO UPDATE SET request_times=ARRAY(SELECT t FROM unnest(revenue_provider_budget.request_times) t WHERE t>now()-interval '24 hours')||now(),last_request=now() WHERE (SELECT count(*) FROM unnest(revenue_provider_budget.request_times) t WHERE t>now()-interval '24 hours')<24 AND revenue_provider_budget.last_request<=now()-interval '13 seconds'`)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() == 1, nil
}
