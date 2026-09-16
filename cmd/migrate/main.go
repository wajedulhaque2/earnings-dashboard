package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"time"

	"earnings-dashboard/internal/config"
	"earnings-dashboard/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if !run(logger) {
		os.Exit(1)
	}
}

func run(logger *slog.Logger) bool {
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	if len(os.Args) > 2 || (command != "up" && command != "down" && command != "status") {
		logger.Error("usage: migrate [up|down|status]")
		return false
	}
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration invalid", "error", err)
		return false
	}
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		logger.Error("database configuration invalid")
		return false
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("database unavailable")
		return false
	}
	goose.SetBaseFS(migrations.Files)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		logger.Error("migration dialect invalid")
		return false
	}
	if command == "status" {
		version, err := goose.GetDBVersionContext(ctx, db)
		if err != nil {
			logger.Error("migration status failed")
			return false
		}
		logger.Info("migration status", "version", version)
		return true
	}
	if err := goose.RunContext(ctx, command, db, "."); err != nil {
		logger.Error("migration failed; verify database permissions and schema")
		return false
	}
	logger.Info("migration complete", "command", command)
	return true
}
