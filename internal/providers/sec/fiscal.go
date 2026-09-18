package sec

import (
	"earnings-dashboard/internal/models"
	"fmt"
)

func labelPeriods(rows []models.Financial, facts factsResponse, s submission) {
	reports := map[string]string{}
	for i, a := range s.Filings.Recent.AccessionNumber {
		if i < len(s.Filings.Recent.ReportDate) {
			reports[a] = s.Filings.Recent.ReportDate[i]
		}
	}
	labels := map[string]fact{}
	ambiguous := map[string]bool{}
	for _, concept := range facts.Facts["us-gaap"] {
		for _, unit := range concept.Units {
			for _, f := range unit {
				if f.FY < 1900 || reports[f.Accn] != f.End || f.End == "" {
					continue
				}
				if f.FP != "Q1" && f.FP != "Q2" && f.FP != "Q3" && f.FP != "FY" {
					continue
				}
				if old, ok := labels[f.End]; !ok || f.Filed > old.Filed {
					labels[f.End] = f
					ambiguous[f.End] = false
				} else if f.Filed == old.Filed && (f.FY != old.FY || f.FP != old.FP) {
					ambiguous[f.End] = true
				}
			}
		}
	}
	for i := range rows {
		if f, ok := labels[rows[i].PeriodEnd.Format("2006-01-02")]; ok && !ambiguous[rows[i].PeriodEnd.Format("2006-01-02")] {
			q := 4
			if f.FP != "FY" {
				q = int(f.FP[1] - '0')
			}
			rows[i].FiscalYear = models.Ptr(f.FY)
			rows[i].FiscalQuarter = models.Ptr(q)
		}
	}
	// If the source reuses a label for multiple directly reported period ends,
	// retain the financial observations but withhold the ambiguous label.
	counts := map[string]int{}
	for _, r := range rows {
		if r.FiscalYear != nil && r.FiscalQuarter != nil {
			counts[fmt.Sprintf("%d/%d", *r.FiscalYear, *r.FiscalQuarter)]++
		}
	}
	for i, r := range rows {
		if r.FiscalYear != nil && r.FiscalQuarter != nil && counts[fmt.Sprintf("%d/%d", *r.FiscalYear, *r.FiscalQuarter)] > 1 {
			rows[i].FiscalYear = nil
			rows[i].FiscalQuarter = nil
		}
	}
}
