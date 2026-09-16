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

// Ready checks the full initial schema, not just the PostgreSQL connection.
func (s *Store) Ready(ctx context.Context) error {
	_, err := s.db.Exec(ctx, `SELECT 1 FROM companies, earnings_events,
 quarterly_financials, earnings_reactions, watchlists,
 watchlist_companies, provider_sync_state, historical_prices, intraday_snapshots, filings, sync_requests LIMIT 0`)
	return err
}
