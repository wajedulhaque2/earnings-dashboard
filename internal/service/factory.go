package service

import (
	"earnings-dashboard/internal/providers"
	"earnings-dashboard/internal/providers/alphavantage"
	"earnings-dashboard/internal/providers/sec"
	"earnings-dashboard/internal/providers/yahoo"
	"log/slog"
)

func New(store Store, secAgent string, logger *slog.Logger, revenueKey ...string) *Sync {
	y := yahoo.New()
	s := sec.New(secAgent)
	syncer := &Sync{Store: store, References: []providers.Reference{y, s}, Fundamentals: []providers.Fundamentals{s, y}, Earnings: y, Market: y, Filings: s, Logger: logger}
	if len(revenueKey) > 0 && revenueKey[0] != "" {
		if cache, ok := store.(alphavantage.Cache); ok {
			syncer.Revenue = alphavantage.New(revenueKey[0], cache)
		}
	}
	return syncer
}
