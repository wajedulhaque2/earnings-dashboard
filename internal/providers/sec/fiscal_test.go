package sec

import (
	"earnings-dashboard/internal/models"
	"testing"
	"time"
)

func TestFiscalLabelsRejectComparativeContext(t *testing.T) {
	var r factsResponse
	r.Facts = map[string]map[string]struct{ Units map[string][]fact }{"us-gaap": {"Revenues": {Units: map[string][]fact{"USD": {
		{Accn: "current", End: "2025-01-31", FY: 2025, FP: "Q1", Filed: "2025-03-01"},
		{Accn: "later", End: "2025-01-31", FY: 2026, FP: "Q1", Filed: "2026-03-01"},
	}}}}}
	var s submission
	s.Filings.Recent.AccessionNumber = []string{"current", "later"}
	s.Filings.Recent.ReportDate = []string{"2025-01-31", "2026-01-31"}
	end, _ := time.Parse("2006-01-02", "2025-01-31")
	rows := []models.Financial{{PeriodEnd: end}}
	labelPeriods(rows, r, s)
	if rows[0].FiscalYear == nil || *rows[0].FiscalYear != 2025 || *rows[0].FiscalQuarter != 1 {
		t.Fatal(rows)
	}
}

func TestReusedFiscalLabelIsWithheld(t *testing.T) {
	var r factsResponse
	r.Facts = map[string]map[string]struct{ Units map[string][]fact }{"us-gaap": {"Revenues": {Units: map[string][]fact{"USD": {
		{Accn: "one", End: "2024-01-31", FY: 2025, FP: "Q1", Filed: "2024-03-01"},
		{Accn: "two", End: "2025-01-31", FY: 2025, FP: "Q1", Filed: "2025-03-01"},
	}}}}}
	var s submission
	s.Filings.Recent.AccessionNumber = []string{"one", "two"}
	s.Filings.Recent.ReportDate = []string{"2024-01-31", "2025-01-31"}
	rows := []models.Financial{}
	for _, end := range s.Filings.Recent.ReportDate {
		d, _ := time.Parse("2006-01-02", end)
		rows = append(rows, models.Financial{PeriodEnd: d, Revenue: models.Ptr(100.0)})
	}
	labelPeriods(rows, r, s)
	for _, row := range rows {
		if row.FiscalYear != nil || row.FiscalQuarter != nil || row.Revenue == nil {
			t.Fatal("ambiguous label retained or numeric observation lost", row)
		}
	}
}
