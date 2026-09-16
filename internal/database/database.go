package database

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Open validates configuration without requiring a live database. Health checks
// report outages, and pgx reconnects when PostgreSQL becomes available.
func Open(ctx context.Context, url string, timeout time.Duration) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	cfg.ConnConfig.ConnectTimeout = timeout
	cfg.ConnConfig.RuntimeParams["timezone"] = "UTC"
	cfg.MaxConns = 10
	cfg.MinConns = 0
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errors.New("could not initialize database pool")
	}
	return pool, nil
}
