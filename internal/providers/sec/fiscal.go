package sec

import "earnings-dashboard/internal/models"

func labelPeriods(rows []models.Financial, facts factsResponse, s submission) {
	for _, p := range fiscalEvidence(facts, s) {
		if p.Ambiguous || p.FiscalQuarter == 0 {
			continue
		}
		for i := range rows {
			if rows[i].PeriodEnd.Equal(p.PeriodEnd) {
				rows[i].FiscalYear = models.Ptr(p.FiscalYear)
				rows[i].FiscalQuarter = models.Ptr(p.FiscalQuarter)
			}
		}
	}
}
