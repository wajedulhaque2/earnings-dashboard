package providers

import (
	"context"
	"earnings-dashboard/internal/models"
	"errors"
	"time"
)

var ErrUnavailable = errors.New("provider data unavailable")
var ErrNotFound = errors.New("symbol not found")
var ErrMalformed = errors.New("invalid provider response")
var ErrUnsupported = errors.New("provider does not support symbol")
var ErrNotConfigured = errors.New("provider not configured")

// Eligibility distinguishes unsupported instruments from temporary provider failures.
type Eligibility interface {
	Eligible(context.Context, string) error
}

type Reference interface {
	Name() string
	Search(context.Context, string) ([]models.Company, error)
	Company(context.Context, string) (models.Company, error)
}
type Earnings interface {
	Name() string
	Earnings(context.Context, string) ([]models.Event, error)
	Calendar(context.Context, time.Time, time.Time) ([]models.CalendarItem, error)
}
type Fundamentals interface {
	Name() string
	Financials(context.Context, string) ([]models.Financial, error)
}
type MarketData interface {
	Name() string
	Prices(context.Context, string) ([]models.Price, error)
	Intraday(context.Context, string) ([]models.Snapshot, error)
}
type Filings interface {
	Filings(context.Context, string) ([]models.Filing, error)
}
