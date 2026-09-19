package earnings

import (
	"earnings-dashboard/internal/models"
	"math"
	"sort"
	"time"
)

func MappingStrength(source string) int {
	switch source {
	case "sec_explicit", "sec_annual_q4":
		return 4
	case "yahoo_event", "sec_same_day":
		return 3
	case "financial_period_match":
		return 2
	case "inferred_fiscal_sequence":
		return 1
	}
	return 0
}

// CompleteSequence infers only an interior missing label bracketed by two
// explicit labels. Both adjacent fiscal periods must be quarter length and the
// labels must advance exactly two quarters, including fiscal-year rollover.
func CompleteSequence(periods []models.FiscalPeriod) []models.FiscalPeriod {
	out := append([]models.FiscalPeriod(nil), periods...)
	sort.Slice(out, func(i, j int) bool { return out[i].PeriodEnd.Before(out[j].PeriodEnd) })
	for i := 1; i+1 < len(out); i++ {
		a, p, b := out[i-1], out[i], out[i+1]
		if p.FiscalQuarter != 0 || p.Ambiguous || a.Ambiguous || b.Ambiguous || a.FiscalQuarter == 0 || b.FiscalQuarter == 0 || MappingStrength(a.Source) < 2 || MappingStrength(b.Source) < 2 {
			continue
		}
		left, right := p.PeriodEnd.Sub(a.PeriodEnd).Hours()/24, b.PeriodEnd.Sub(p.PeriodEnd).Hours()/24
		if left < 75 || left > 105 || right < 75 || right > 105 {
			continue
		}
		serial := a.FiscalYear*4 + a.FiscalQuarter - 1
		if b.FiscalYear*4+b.FiscalQuarter-1 != serial+2 {
			continue
		}
		p.FiscalYear = (serial + 1) / 4
		p.FiscalQuarter = (serial+1)%4 + 1
		p.Source = "inferred_fiscal_sequence"
		out[i] = p
	}
	return out
}

// MatchFiscal normally requires an event 20–120 days after its period end and a filing
// no more than 45 days from that event. Nearest filing wins only with a clear
// (>7 day) margin; otherwise competing periods remain ambiguous. Early reporters
// may match at 7–19 days with explicit SEC evidence and a filing within 30 days,
// or a directly labeled quarterly financial record. Unfiled records are ranked
// by period-end proximity, so an older quarter cannot displace a newer one.
func MatchFiscal(e models.Event, periods []models.FiscalPeriod) (*models.FiscalPeriod, string) {
	type candidate struct {
		p        models.FiscalPeriod
		distance float64
	}
	candidates := []candidate{}
	for _, p := range periods {
		days := e.ReportDate.Sub(p.PeriodEnd).Hours() / 24
		if days < 7 || days > 120 {
			continue
		}
		if e.PeriodEnd != nil && !e.PeriodEnd.Equal(p.PeriodEnd) {
			gap := e.PeriodEnd.Sub(p.PeriodEnd).Hours() / 24
			if gap < 0 {
				gap = -gap
			}
			if MappingStrength(textValue(e.FiscalLabelSource)) >= MappingStrength(p.Source) || gap > 7 {
				continue
			}
		}
		if MappingStrength(textValue(e.FiscalLabelSource)) >= MappingStrength(p.Source) && ((e.FiscalYear != nil && p.FiscalYear != *e.FiscalYear) || (e.FiscalQuarter != nil && p.FiscalQuarter != *e.FiscalQuarter)) {
			continue
		}
		distance := p.Filed.Sub(e.ReportDate).Hours() / 24
		if distance < 0 {
			distance = -distance
		}
		if !p.Filed.IsZero() && distance > 45 {
			continue
		}
		directFinancial := p.Source == "financial_period_match" && p.FiscalYear > 0 && p.FiscalQuarter > 0 && p.Filed.IsZero()
		if days < 20 && !directFinancial && (MappingStrength(p.Source) != 4 || p.Filed.IsZero() || distance > 30) {
			continue
		}
		if p.Filed.IsZero() {
			distance = days
		}
		candidates = append(candidates, candidate{p, distance})
	}
	if len(candidates) == 0 {
		return nil, "no_matching_filing"
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].distance < candidates[j].distance })
	best := candidates[0]
	if best.p.Ambiguous {
		return nil, "ambiguous_period"
	}
	if len(candidates) > 1 && candidates[1].distance-best.distance <= 7 {
		return nil, "ambiguous_period"
	}
	if best.p.FiscalQuarter == 0 {
		return nil, "insufficient_history"
	}
	return &best.p, ""
}

func textValue(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// HistoricalRevenueEstimate carries identity and point-in-time evidence. An
// observation from a current forward-looking trend is never historical evidence.
type HistoricalRevenueEstimate struct {
	Symbol, Source, Currency          string
	FiscalYear, FiscalQuarter         int
	PeriodEnd, ReportDate, ObservedAt time.Time
	Value                             *float64
	Historical                        bool
	ReportedQuarter                   bool // independently corroborated closed-quarter history
}

func MatchRevenueEstimate(e models.Event, v HistoricalRevenueEstimate) bool {
	return v.Historical && v.Source != "" && v.Value != nil && !math.IsNaN(*v.Value) && !math.IsInf(*v.Value, 0) && v.Symbol == e.Symbol &&
		e.FiscalYear != nil && e.FiscalQuarter != nil && e.PeriodEnd != nil &&
		v.FiscalYear == *e.FiscalYear && v.FiscalQuarter == *e.FiscalQuarter && v.PeriodEnd.Equal(*e.PeriodEnd) &&
		v.ReportDate.Equal(e.ReportDate) && ((!v.ObservedAt.IsZero() && v.ObservedAt.Before(e.ReportDate)) || (v.Source == "alphavantage" && v.ReportedQuarter && v.ObservedAt.IsZero()))
}
