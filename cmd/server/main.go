package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"earnings-dashboard/internal/config"
	"earnings-dashboard/internal/database"
	"earnings-dashboard/internal/repository"
	"earnings-dashboard/internal/web"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseTimeout)
	if err != nil {
		return err
	}
	defer pool.Close()
	server := &http.Server{
		Addr:              cfg.Address(),
		Handler:           web.NewRouter(repository.New(pool), cfg.DatabaseTimeout, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      cfg.DatabaseTimeout + 10*time.Second,
		IdleTimeout:       60 * time.Second,
	}
	failures := make(chan error, 1)
	go func() { failures <- server.ListenAndServe() }()
	logger.Info("server starting", "port", cfg.Port, "environment", cfg.Environment)
	select {
	case err := <-failures:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP listener failed")
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return errors.New("HTTP shutdown timed out")
		}
		return nil
	}
}
