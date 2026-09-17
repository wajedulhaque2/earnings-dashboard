package service

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
	"earnings-dashboard/internal/providers/httpclient"
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
	RecordSync(context.Context, string, string, string, ...string) error
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
	if errors.Is(err, providers.ErrUnsupported) {
		status = "unsupported"
	} else if errors.Is(err, providers.ErrNotConfigured) {
		status, category = "not_attempted", "not_configured"
	} else if err != nil {
		status, category = "error", errorCategory(err)
	}
	stateErr := s.Store.RecordSync(ctx, provider, operation+":"+symbol, status, category)
	s.Logger.Info("provider refresh", "provider", provider, "operation", operation, "symbol", symbol, "duration", time.Since(start), "status", status, "error", category)
	if stateErr != nil {
		return stateErr
	}
	return err
}

func errorCategory(err error) string {
	var status *httpclient.StatusError
	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, providers.ErrMalformed):
		return "invalid_response"
	case errors.Is(err, providers.ErrNotFound):
		return "not_found"
	case errors.Is(err, providers.ErrUnavailable):
		return "data_unavailable"
	case errors.As(err, &status):
		switch status.Code {
		case 401, 403:
			return "access_denied"
		case 404:
			return "not_found"
		case 429:
			return "rate_limited"
		default:
			return "provider_http_error"
		}
	default:
		return "refresh_failed"
	}
}
func eligible(ctx context.Context, provider any, symbol string) error {
	if p, ok := provider.(providers.Eligibility); ok {
		return p.Eligible(ctx, symbol)
	}
	return nil
}
func (s *Sync) SyncCompany(ctx context.Context, symbol string) error {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return err
	}
	var failures []error
	for _, p := range s.References {
		err = s.operation(ctx, p.Name(), "company", symbol, func() error {
			if e := eligible(ctx, p, symbol); e != nil {
				return e
			}
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
			if e := eligible(ctx, p, c.Symbol); e != nil {
				return e
			}
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
		} else if !errors.Is(err, providers.ErrUnsupported) {
			failures = append(failures, err)
		}
	}
	if s.Filings != nil {
		_ = s.operation(ctx, "sec", "filings", c.Symbol, func() error {
			if e := eligible(ctx, s.Filings, c.Symbol); e != nil {
				return e
			}
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
