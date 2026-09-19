package sec

import (
	"context"
	"earnings-dashboard/internal/models"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExplicitAndAnnualEvidence(t *testing.T) {
	for _, tc := range []struct {
		fp, form, start string
		want            int
	}{
		{"Q1", "10-Q", "2025-02-01", 1}, {"Q2", "10-Q/A", "2024-11-01", 2}, {"Q3", "10-Q", "2024-08-01", 3},
		{"FY", "10-K", "2024-05-01", 4}, {"FY", "10-K/A", "2024-05-01", 4},
		{"FY", "10-K", "2025-02-01", 0}, {"FY", "10-Q", "2024-05-01", 0},
	} {
		t.Run(tc.fp+tc.form+tc.start, func(t *testing.T) {
			var s submission
			s.Filings.Recent = filingList{AccessionNumber: []string{"a"}, Form: []string{tc.form}, ReportDate: []string{"2025-04-30"}, FilingDate: []string{"2025-05-25"}}
			facts := factsResponse{Facts: map[string]map[string]struct{ Units map[string][]fact }{"us-gaap": {"Revenues": {Units: map[string][]fact{"USD": {{Accn: "a", FY: 2025, FP: tc.fp, Start: tc.start, End: "2025-04-30", Form: tc.form, Val: models.Ptr(100.0)}}}}}}}
			got := fiscalEvidence(facts, s)
			if len(got) != 1 || got[0].FiscalQuarter != tc.want {
				t.Fatal(got)
			}
			if tc.want == 4 && got[0].Source != "sec_annual_q4" {
				t.Fatal(got)
			}
		})
	}
}

func TestHistoricalSubmissionArchives(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/files/company_tickers.json":
			w.Write([]byte(`{"0":{"cik_str":1,"ticker":"TEST"}}`))
		case "/submissions/CIK0000000001.json":
			w.Write([]byte(`{"filings":{"recent":{},"files":[{"name":"CIK0000000001-submissions-001.json","filingTo":"2099-01-01"}]}}`))
		case "/submissions/CIK0000000001-submissions-001.json":
			calls++
			json.NewEncoder(w).Encode(filingList{AccessionNumber: []string{"old"}, Form: []string{"10-Q/A"}, FilingDate: []string{"2018-05-20"}, ReportDate: []string{"2018-03-31"}, PrimaryDocument: []string{"q.htm"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New("test test@test.invalid")
	c.DataBase = srv.URL
	c.WWWBase = srv.URL
	rows, err := c.Filings(context.Background(), "TEST")
	if err != nil || len(rows) != 1 || rows[0].Accession != "old" || calls != 1 {
		t.Fatal(rows, err, calls)
	}
}

func TestFiscalLabelsRejectComparativeContext(t *testing.T) {
	var r factsResponse
	r.Facts = map[string]map[string]struct{ Units map[string][]fact }{"us-gaap": {"Revenues": {Units: map[string][]fact{"USD": {
		{Accn: "current", End: "2025-01-31", FY: 2025, FP: "Q1", Filed: "2025-03-01"},
		{Accn: "later", End: "2025-01-31", FY: 2026, FP: "Q1", Filed: "2026-03-01"},
	}}}}}
	var s submission
	s.Filings.Recent.AccessionNumber = []string{"current", "later"}
	s.Filings.Recent.Form = []string{"10-Q", "10-Q"}
	s.Filings.Recent.FilingDate = []string{"2025-03-01", "2026-03-01"}
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
	s.Filings.Recent.Form = []string{"10-Q", "10-Q"}
	s.Filings.Recent.FilingDate = []string{"2025-03-01", "2026-03-01"}
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
