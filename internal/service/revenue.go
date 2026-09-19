package service

import (
	"context"
	"earnings-dashboard/internal/earnings"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
)

// Revenue consensus is optional and only refreshed by workers. Restrict the
// currency-less AV endpoint to domestic issuers with established USD reporting.
func (s *Sync) SyncRevenueEstimates(ctx context.Context, symbol string) error {
	if s.Revenue == nil {
		return providers.ErrNotConfigured
	}
	c, err := s.company(ctx, symbol)
	if err != nil {
		return err
	}
	if !c.UniverseEligible || c.Country == nil || *c.Country != "United States" {
		return nil
	}
	store, ok := s.Store.(interface {
		USDRevenueHistory(context.Context, int64) (bool, error)
		Events(context.Context, int64) ([]models.Event, error)
		StoreHistoricalRevenueEstimate(context.Context, models.Event, earnings.HistoricalRevenueEstimate) error
		LinkFiscalLabels(context.Context, int64) error
	})
	if !ok {
		return providers.ErrUnavailable
	}
	usd, err := store.USDRevenueHistory(ctx, c.ID)
	if err != nil {
		return err
	}
	if !usd {
		return nil
	}
	events, err := store.Events(ctx, c.ID)
	if err != nil {
		return err
	}
	needed := false
	for _, e := range events {
		if e.RevenueEstimate == nil && e.FiscalYear != nil && e.FiscalQuarter != nil && e.PeriodEnd != nil {
			needed = true
		}
	}
	if !needed {
		return nil
	}
	return s.operation(ctx, s.Revenue.Name(), "revenue_estimates", c.Symbol, func() error {
		rows, err := s.Revenue.HistoricalRevenue(ctx, c.Symbol)
		if err != nil {
			return err
		}
		for _, e := range events {
			if e.FiscalYear == nil || e.FiscalQuarter == nil || e.PeriodEnd == nil {
				continue
			}
			for _, r := range rows {
				v := earnings.HistoricalRevenueEstimate{Symbol: r.Symbol, Source: r.Source, Currency: "USD", FiscalYear: *e.FiscalYear, FiscalQuarter: *e.FiscalQuarter, PeriodEnd: r.PeriodEnd, ReportDate: r.ReportDate, Value: models.Ptr(r.Value), Historical: true, ReportedQuarter: true}
				if earnings.MatchRevenueEstimate(e, v) {
					if err = store.StoreHistoricalRevenueEstimate(ctx, e, v); err != nil {
						return err
					}
				}
			}
		}
		return store.LinkFiscalLabels(ctx, c.ID)
	})
}
