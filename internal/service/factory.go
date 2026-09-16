package service

import (
	"earnings-dashboard/internal/providers"
	"earnings-dashboard/internal/providers/sec"
	"earnings-dashboard/internal/providers/yahoo"
	"log/slog"
)

func New(store Store, secAgent string, logger *slog.Logger) *Sync {
	y := yahoo.New()
	s := sec.New(secAgent)
	return &Sync{Store: store, References: []providers.Reference{y, s}, Fundamentals: []providers.Fundamentals{s, y}, Earnings: y, Market: y, Filings: s, Logger: logger}
}
