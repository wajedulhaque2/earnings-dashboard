// The Phase 1 worker performs a single database readiness check. Scheduling and
// provider ingestion are deliberately deferred to later phases.
package main

import (
	"context"
	"log/slog"
	"os"

	"earnings-dashboard/internal/config"
	"earnings-dashboard/internal/database"
	"earnings-dashboard/internal/repository"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if !run(logger) {
		os.Exit(1)
	}
}

func run(logger *slog.Logger) bool {
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration invalid", "error", err)
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DatabaseTimeout)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseTimeout)
	if err != nil {
		logger.Error("database configuration invalid")
		return false
	}
	defer pool.Close()
	if err := repository.New(pool).Ready(ctx); err != nil {
		logger.Error("database unavailable or migrations missing")
		return false
	}
	logger.Info("worker readiness check complete; no ingestion jobs in Phase 1")
	return true
}
