// Worker runs explicit, idempotent refresh operations.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"earnings-dashboard/internal/config"
	"earnings-dashboard/internal/database"
	"earnings-dashboard/internal/repository"
	"earnings-dashboard/internal/scheduler"
	"earnings-dashboard/internal/service"
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
	root, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(root, 30*time.Minute)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseTimeout)
	if err != nil {
		logger.Error("database configuration invalid")
		return false
	}
	defer pool.Close()
	readyCtx, readyCancel := context.WithTimeout(ctx, cfg.DatabaseTimeout)
	readyErr := repository.New(pool).Ready(readyCtx)
	readyCancel()
	if readyErr != nil {
		logger.Error("database unavailable or migrations missing")
		return false
	}
	store := repository.New(pool)
	syncer := service.New(store, cfg.SECUserAgent, logger)
	command := "check"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	symbol := ""
	if len(os.Args) > 2 {
		symbol = os.Args[2]
	}
	switch command {
	case "check":
		return true
	case "schedule":
		// Hold an advisory lock on one dedicated connection for this process.
		// A second scheduler exits rather than duplicating provider traffic.
		conn, e := pool.Acquire(root)
		if e != nil {
			return false
		}
		defer conn.Release()
		var locked bool
		if e = conn.QueryRow(root, "SELECT pg_try_advisory_lock(734821901)").Scan(&locked); e != nil || !locked {
			logger.Error("another scheduler is running or lock unavailable")
			return false
		}
		defer func() {
			release, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = conn.Exec(release, "SELECT pg_advisory_unlock(734821901)")
		}()
		job := scheduler.Scheduler{Store: store, Sync: syncer, Logger: logger, Location: cfg.MarketLocation}
		err = job.Run(root)
	case "sync-company":
		err = syncer.SyncCompany(ctx, symbol)
	case "sync-financials":
		err = syncer.SyncFinancials(ctx, symbol)
	case "sync-earnings":
		err = syncer.SyncEarnings(ctx, symbol)
	case "sync-prices":
		err = syncer.SyncHistoricalPrices(ctx, symbol)
	case "sync-intraday":
		err = syncer.SyncRecentIntraday(ctx, symbol)
	case "sync-calendar":
		now := time.Now().In(cfg.MarketLocation)
		err = syncer.SyncUpcomingCalendar(ctx, now.AddDate(0, 0, -7), now.AddDate(0, 0, 14))
	case "sync-all":
		err = syncer.SyncAll(ctx, symbol)
	case "recalculate":
		err = service.Recalculate(ctx, store, symbol, time.Now())
	default:
		logger.Error("unknown worker command")
		return false
	}
	if command == "sync-all" || command == "sync-prices" || command == "sync-earnings" || command == "sync-intraday" {
		calcErr := service.Recalculate(ctx, store, symbol, time.Now())
		if err == nil {
			err = calcErr
		}
	}
	if err != nil {
		logger.Error("worker operation incomplete", "operation", command, "status", "error")
		return false
	}
	return true
}
