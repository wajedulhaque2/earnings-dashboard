package earnings

import (
	"earnings-dashboard/internal/models"
	"testing"
	"time"
)

func fiscalDate(s string) time.Time {
	d, e := time.Parse("2006-01-02", s)
	if e != nil {
		panic(e)
	}
	return d
}
func TestMatchFiscalBoundsAndAmbiguity(t *testing.T) {
	period := models.FiscalPeriod{PeriodEnd: fiscalDate("2025-04-27"), Filed: fiscalDate("2025-05-29"), FiscalYear: 2026, FiscalQuarter: 1, Source: "sec_explicit"}
	for _, tc := range []struct {
		name, date string
		other      *models.FiscalPeriod
		want       bool
		reason     string
	}{
		{"noncalendar", "2025-05-28", nil, true, ""},
		{"early explicit", "2025-05-09", nil, true, ""},
		{"too early", "2025-05-03", nil, false, "no_matching_filing"},
		{"before end", "2025-04-26", nil, false, "no_matching_filing"},
		{"too late", "2025-09-01", nil, false, "no_matching_filing"},
		{"competing period", "2025-05-28", &models.FiscalPeriod{PeriodEnd: fiscalDate("2025-02-01"), Filed: fiscalDate("2025-05-30"), FiscalYear: 2025, FiscalQuarter: 4, Source: "sec_annual_q4"}, false, "ambiguous_period"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ps := []models.FiscalPeriod{period}
			if tc.other != nil {
				ps = append(ps, *tc.other)
			}
			p, r := MatchFiscal(models.Event{ReportDate: fiscalDate(tc.date)}, ps)
			if (p != nil) != tc.want || r != tc.reason {
				t.Fatal(p, r)
			}
			if p != nil && (p.FiscalYear != 2026 || p.FiscalQuarter != 1) {
				t.Fatal("calendar year substituted", p)
			}
		})
	}
	period.Ambiguous = true
	p, r := MatchFiscal(models.Event{ReportDate: fiscalDate("2025-05-28")}, []models.FiscalPeriod{period})
	if p != nil || r != "ambiguous_period" {
		t.Fatal(p, r)
	}
}

func TestFiscalSequence(t *testing.T) {
	ps := []models.FiscalPeriod{{PeriodEnd: fiscalDate("2024-10-27"), FiscalYear: 2025, FiscalQuarter: 3, Source: "sec_explicit"}, {PeriodEnd: fiscalDate("2025-01-26")}, {PeriodEnd: fiscalDate("2025-04-27"), FiscalYear: 2026, FiscalQuarter: 1, Source: "sec_explicit"}}
	got := CompleteSequence(ps)
	if got[1].FiscalYear != 2025 || got[1].FiscalQuarter != 4 || got[1].Source != "inferred_fiscal_sequence" {
		t.Fatal(got)
	}
	ps[2].FiscalQuarter = 2
	if CompleteSequence(ps)[1].FiscalQuarter != 0 {
		t.Fatal("inferred across conflicting sequence")
	}
	ps[2].FiscalQuarter = 1
	ps[1].Ambiguous = true
	if CompleteSequence(ps)[1].FiscalQuarter != 0 {
		t.Fatal("ambiguity overwritten")
	}
	ps[1].Ambiguous = false
	ps[1].PeriodEnd = fiscalDate("2024-11-30")
	if CompleteSequence(ps)[1].FiscalQuarter != 0 {
		t.Fatal("nonquarter duration inferred")
	}
}

func TestEarlyFinancialPeriodDoesNotBorrowPreviousQuarter(t *testing.T) {
	ps := []models.FiscalPeriod{{PeriodEnd: fiscalDate("2025-03-31"), FiscalYear: 2025, FiscalQuarter: 1, Source: "financial_period_match"}, {PeriodEnd: fiscalDate("2025-06-30"), FiscalYear: 2025, FiscalQuarter: 2, Source: "financial_period_match"}}
	p, reason := MatchFiscal(models.Event{ReportDate: fiscalDate("2025-07-15")}, ps)
	if p == nil || p.FiscalQuarter != 2 || reason != "" {
		t.Fatal("early report matched prior quarter", p, reason)
	}
}

func TestHistoricalRevenueIdentity(t *testing.T) {
	e := models.Event{Symbol: "TEST", FiscalYear: models.Ptr(2025), FiscalQuarter: models.Ptr(1), PeriodEnd: models.Ptr(fiscalDate("2025-03-31")), ReportDate: fiscalDate("2025-05-01")}
	v := HistoricalRevenueEstimate{Symbol: "TEST", FiscalYear: 2025, FiscalQuarter: 1, PeriodEnd: *e.PeriodEnd, ReportDate: e.ReportDate, ObservedAt: fiscalDate("2025-04-30"), Value: models.Ptr(100.0), Source: "yahoo", Currency: "USD", Historical: true}
	if !MatchRevenueEstimate(e, v) {
		t.Fatal("historical estimate rejected")
	}
	for _, mutate := range []func(*HistoricalRevenueEstimate){func(v *HistoricalRevenueEstimate) { v.Symbol = "OTHER" }, func(v *HistoricalRevenueEstimate) { v.FiscalYear++ }, func(v *HistoricalRevenueEstimate) { v.FiscalQuarter++ }, func(v *HistoricalRevenueEstimate) { v.PeriodEnd = v.PeriodEnd.AddDate(0, 0, 1) }, func(v *HistoricalRevenueEstimate) { v.ReportDate = v.ReportDate.AddDate(0, 0, 1) }, func(v *HistoricalRevenueEstimate) { v.ObservedAt = v.ReportDate }, func(v *HistoricalRevenueEstimate) { v.Historical = false }, func(v *HistoricalRevenueEstimate) { v.Value = nil }} {
		copy := v
		mutate(&copy)
		if MatchRevenueEstimate(e, copy) {
			t.Fatal("incorrect or forward estimate accepted", copy)
		}
	}
}
