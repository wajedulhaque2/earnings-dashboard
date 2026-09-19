// Package models contains normalized domain data. Nil always means unavailable.
package models

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var symbolPattern = regexp.MustCompile(`^[A-Z0-9^][A-Z0-9.^=_-]{0,31}$`)

func Symbol(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if !symbolPattern.MatchString(s) {
		return "", errors.New("invalid symbol")
	}
	return s, nil
}
func Ptr[T any](v T) *T { return &v }
func Text(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

type Company struct {
	FiscalYearEnd                                                                            *string
	SecurityType, ExchangeCode                                                               *string
	UniverseEligible                                                                         bool
	UniverseReason                                                                           string
	UniverseCheckedAt                                                                        *time.Time
	ID                                                                                       int64
	Symbol                                                                                   string
	Name, Exchange, Country, Sector, Industry, Description, LogoURL, Currency, Timezone, CIK *string
	MarketCap, LatestPrice                                                                   *float64
	QuoteTime                                                                                *time.Time
	UpdatedAt                                                                                time.Time
}
type Event struct {
	FiscalLabelSource, FiscalMappingReason                              *string
	RevenueActualSource, RevenueEstimateSource                          *string
	ID, CompanyID                                                       int64
	Symbol                                                              string
	ReportDate                                                          time.Time
	ReportTime                                                          *time.Time
	FiscalYear, FiscalQuarter                                           *int
	PeriodEnd                                                           *time.Time
	Session, Source                                                     string
	EPSEstimate, EPSActual, EPSSurprise, EPSSurprisePct                 *float64
	RevenueEstimate, RevenueActual, RevenueSurprise, RevenueSurprisePct *float64
}

// FiscalPeriod retains SEC filing context, separately from quarterly amounts.
// An annual period is evidence for a label, never a quarterly revenue value.
type FiscalPeriod struct {
	CompanyID                 int64
	PeriodEnd, Filed          time.Time
	FiscalYear, FiscalQuarter int
	Source, Accession, Form   string
	Ambiguous                 bool
}

// RevenueConsensus is a closed reported quarter, cross-checked against the
// provider's historical earnings record. It is not a current forecast snapshot.
type RevenueConsensus struct {
	Symbol, Source        string
	PeriodEnd, ReportDate time.Time
	Value                 float64
}
type Financial struct {
	CompanyID                 int64
	FiscalYear, FiscalQuarter *int
	PeriodEnd                 time.Time
	Revenue, DilutedEPS       *float64
	Currency, Source          string
	RevenueSource, EPSSource  *string
}
type Price struct {
	CompanyID                             int64
	Date                                  time.Time
	Open, High, Low, Close, AdjustedClose *float64
	Volume                                *int64
	Source                                string
	SplitRatio                            *float64
}
type Snapshot struct {
	CompanyID       int64
	Time            time.Time
	Session, Source string
	Price           float64
}
type Filing struct {
	CompanyID                 int64
	Accession, Form, Document string
	Filed, ReportDate         time.Time
}
type CalendarItem struct {
	Company Company
	Event   Event
}
type Reaction struct {
	PremarketReturn                                      *float64
	EventID                                              int64
	PreviousClose, PremarketPrice, EventOpen, EventClose *float64
	Returns                                              [8]*float64 // opening gap, event, 1D, 2D, 1W, 2W, 1M, 3M (fractional)
	EventDate                                            *time.Time
	Methodology                                          string
}
type History struct {
	Event    Event
	Reaction Reaction
}
type SyncState struct {
	Provider, Operation, Symbol string
	LatestAttemptAt             *time.Time
	LatestAttemptStatus         string
	LatestErrorCategory         *string
	LastSuccessAt               *time.Time
}
