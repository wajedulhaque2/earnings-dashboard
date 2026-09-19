package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB allows repositories to use either a pool or a transaction.
type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

type Store struct{ db DB }

func New(db DB) *Store { return &Store{db: db} }

// Ready checks tables and columns required by the current application.
func (s *Store) Ready(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `SELECT companies.quote_time, companies.timezone,companies.universe_eligible,companies.fiscal_year_end,earnings_reactions.opening_gap_pct,earnings_events.fiscal_label_source,canonical_quarterly_financials.period_end,
 earnings_events.period_end, quarterly_financials.revenue_source,
 quarterly_financials.eps_source, earnings_reactions.event_date,
 historical_prices.split_ratio, provider_sync_state.latest_attempt_at,earnings_events.revenue_estimate_source,earnings_events.fiscal_mapping_reason,fiscal_periods.mapping_source,revenue_provider_cache.fetched_at FROM companies, earnings_events,fiscal_periods,revenue_provider_cache,revenue_provider_budget,
 quarterly_financials,canonical_quarterly_financials, earnings_reactions, watchlists,
 watchlist_companies, provider_sync_state, historical_prices, intraday_snapshots, filings, sync_requests LIMIT 0`)
	return err
}
