package repository

import (
	"context"
)

// DueSymbols uses persisted attempt times so unavailable sources do not create
// hot retry loops. Watchlist companies take priority, without a fixed universe.
func (s *Store) DueSymbols(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT c.symbol FROM companies c LEFT JOIN provider_sync_state p ON p.provider='yahoo' AND p.resource='all:'||c.symbol
 WHERE c.active AND (p.latest_attempt_at IS NULL OR p.latest_attempt_at<now()-interval '24 hours')
 ORDER BY EXISTS(SELECT 1 FROM watchlist_companies w WHERE w.company_id=c.id) DESC,p.latest_attempt_at NULLS FIRST,c.symbol LIMIT 3`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) CalendarDue(ctx context.Context) (bool, error) {
	var due bool
	err := s.db.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM provider_sync_state WHERE provider='yahoo' AND resource='calendar:' AND latest_attempt_at>now()-interval '6 hours')`).Scan(&due)
	return due, err
}
func (s *Store) IntradaySymbols(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT c.symbol FROM companies c JOIN earnings_events e ON e.company_id=c.id
 WHERE c.timezone='America/New_York' AND e.report_date BETWEEN (now() AT TIME ZONE 'America/New_York')::date-7 AND (now() AT TIME ZONE 'America/New_York')::date
 AND NOT EXISTS(SELECT 1 FROM provider_sync_state p WHERE p.provider='yahoo' AND p.resource='intraday:'||c.symbol AND p.latest_attempt_at>now()-interval '1 hour')
 ORDER BY c.symbol LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
