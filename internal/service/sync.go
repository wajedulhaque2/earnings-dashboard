package service

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
	"errors"
	"log/slog"
	"time"
)

type Store interface {
	Company(context.Context, string) (models.Company, error)
	UpsertCompany(context.Context, models.Company) (models.Company, error)
	UpsertEvent(context.Context, models.Event) error
	UpsertFinancial(context.Context, models.Financial) error
	UpsertPrice(context.Context, models.Price) error
	UpsertSnapshot(context.Context, models.Snapshot) error
	UpsertFiling(context.Context, models.Filing) error
	RecordSync(context.Context, string, string, string) error
}
type Sync struct {
	Store        Store
	References   []providers.Reference
	Fundamentals []providers.Fundamentals
	Earnings     providers.Earnings
	Market       providers.MarketData
	Filings      providers.Filings
	Logger       *slog.Logger
}

func (s *Sync) operation(ctx context.Context, provider, operation, symbol string, fn func() error) error {
	start := time.Now()
	err := fn()
	status := "success"
	category := ""
	if err != nil {
		status = "error"
		category = "refresh_failed"
	}
	stateErr := s.Store.RecordSync(ctx, provider, operation+":"+symbol, status)
	s.Logger.Info("provider refresh", "provider", provider, "operation", operation, "symbol", symbol, "duration", time.Since(start), "status", status, "error", category)
	return errors.Join(err, stateErr)
}
func (s *Sync) SyncCompany(ctx context.Context, symbol string) error {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return err
	}
	var failures []error
	for _, p := range s.References {
		err = s.operation(ctx, p.Name(), "company", symbol, func() error {
			v, e := p.Company(ctx, symbol)
			if e != nil {
				return e
			}
			if v.Symbol != symbol {
				return providers.ErrMalformed
			}
			_, e = s.Store.UpsertCompany(ctx, v)
			return e
		})
		if err == nil {
			return nil
		}
		failures = append(failures, err)
	}
	return errors.Join(failures...)
}
func (s *Sync) company(ctx context.Context, symbol string) (models.Company, error) {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return models.Company{}, err
	}
	c, err := s.Store.Company(ctx, symbol)
	if err == nil {
		return c, nil
	}
	if err = s.SyncCompany(ctx, symbol); err != nil {
		return c, err
	}
	return s.Store.Company(ctx, symbol)
}
func (s *Sync) Search(ctx context.Context, q string) ([]models.Company, error) {
	var failures []error
	for _, p := range s.References {
		var found []models.Company
		err := s.operation(ctx, p.Name(), "search", "", func() error {
			var e error
			found, e = p.Search(ctx, q)
			if e != nil {
				return e
			}
			for i, v := range found {
				found[i], e = s.Store.UpsertCompany(ctx, v)
				if e != nil {
					return e
				}
			}
			return nil
		})
		if err == nil && len(found) > 0 {
			return found, nil
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	return nil, errors.Join(failures...)
}
func (s *Sync) SyncFinancials(ctx context.Context, symbol string) error {
	c, err := s.company(ctx, symbol)
	if err != nil {
		return err
	}
	success := false
	var failures []error
	for _, p := range s.Fundamentals {
		err = s.operation(ctx, p.Name(), "financials", c.Symbol, func() error {
			rows, e := p.Financials(ctx, c.Symbol)
			if e != nil {
				return e
			}
			if len(rows) == 0 {
				return providers.ErrUnavailable
			}
			for _, v := range rows {
				v.CompanyID = c.ID
				if e = s.Store.UpsertFinancial(ctx, v); e != nil {
					return e
				}
			}
			return nil
		})
		if err == nil {
			success = true
		} else {
			failures = append(failures, err)
		}
	}
	if s.Filings != nil {
		_ = s.operation(ctx, "sec", "filings", c.Symbol, func() error {
			rows, e := s.Filings.Filings(ctx, c.Symbol)
			if e != nil {
				return e
			}
			for _, v := range rows {
				v.CompanyID = c.ID
				if e = s.Store.UpsertFiling(ctx, v); e != nil {
					return e
				}
			}
			return nil
		})
	}
	if success {
		return nil
	}
	return errors.Join(failures...)
}
func (s *Sync) SyncEarnings(ctx context.Context, symbol string) error {
	c, err := s.company(ctx, symbol)
	if err != nil {
		return err
	}
	return s.operation(ctx, s.Earnings.Name(), "earnings", c.Symbol, func() error {
		rows, e := s.Earnings.Earnings(ctx, c.Symbol)
		if e != nil {
			return e
		}
		for _, v := range rows {
			v.CompanyID = c.ID
			if e = s.Store.UpsertEvent(ctx, v); e != nil {
				return e
			}
		}
		return nil
	})
}
func (s *Sync) SyncHistoricalPrices(ctx context.Context, symbol string) error {
	c, err := s.company(ctx, symbol)
	if err != nil {
		return err
	}
	return s.operation(ctx, s.Market.Name(), "prices", c.Symbol, func() error {
		rows, e := s.Market.Prices(ctx, c.Symbol)
		if e != nil {
			return e
		}
		for _, v := range rows {
			v.CompanyID = c.ID
			if e = s.Store.UpsertPrice(ctx, v); e != nil {
				return e
			}
		}
		return nil
	})
}
func (s *Sync) SyncRecentIntraday(ctx context.Context, symbol string) error {
	c, err := s.company(ctx, symbol)
	if err != nil {
		return err
	}
	return s.operation(ctx, s.Market.Name(), "intraday", c.Symbol, func() error {
		rows, e := s.Market.Intraday(ctx, c.Symbol)
		if e != nil {
			return e
		}
		for _, v := range rows {
			v.CompanyID = c.ID
			if e = s.Store.UpsertSnapshot(ctx, v); e != nil {
				return e
			}
		}
		return nil
	})
}
func (s *Sync) SyncUpcomingCalendar(ctx context.Context, from, to time.Time) error {
	if to.Before(from) || to.Sub(from) > 31*24*time.Hour {
		return errors.New("calendar window must be at most 31 days")
	}
	return s.operation(ctx, s.Earnings.Name(), "calendar", "", func() error {
		rows, err := s.Earnings.Calendar(ctx, from, to)
		if err != nil {
			return err
		}
		for _, v := range rows {
			c, e := s.Store.UpsertCompany(ctx, v.Company)
			if e != nil {
				return e
			}
			v.Event.CompanyID = c.ID
			if e = s.Store.UpsertEvent(ctx, v.Event); e != nil {
				return e
			}
		}
		return nil
	})
}
func (s *Sync) SyncAll(ctx context.Context, symbol string) error {
	var errs []error
	for _, fn := range []func(context.Context, string) error{s.SyncCompany, s.SyncFinancials, s.SyncEarnings, s.SyncHistoricalPrices, s.SyncRecentIntraday} {
		if err := fn(ctx, symbol); err != nil {
			errs = append(errs, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(errs...)
}
