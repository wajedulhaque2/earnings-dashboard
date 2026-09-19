package sec

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
	"strings"
	"time"
)

func (c *Client) historicalSubmission(ctx context.Context, symbol string) (submission, error) {
	_, s, err := c.submission(ctx, symbol)
	if err != nil {
		return s, err
	}
	cutoff := time.Now().AddDate(-12, 0, 0).Format("2006-01-02")
	for _, file := range s.Filings.Files {
		if file.FilingTo < cutoff {
			continue
		}
		if !strings.HasPrefix(file.Name, "CIK") || !strings.HasSuffix(file.Name, ".json") || strings.ContainsAny(file.Name, "/\\") {
			return s, providers.ErrMalformed
		}
		var old filingList
		if err = c.HTTP.JSON(ctx, "GET", c.DataBase+"/submissions/"+file.Name, nil, &old); err != nil {
			return s, err
		}
		r := &s.Filings.Recent
		r.AccessionNumber = append(r.AccessionNumber, old.AccessionNumber...)
		r.Form = append(r.Form, old.Form...)
		r.FilingDate = append(r.FilingDate, old.FilingDate...)
		r.ReportDate = append(r.ReportDate, old.ReportDate...)
		r.PrimaryDocument = append(r.PrimaryDocument, old.PrimaryDocument...)
	}
	return s, nil
}

// FiscalPeriods normally reuses the evidence just fetched by Financials. Keeping
// only one company's evidence bounds memory during a universe refresh.
func (c *Client) FiscalPeriods(ctx context.Context, symbol string) ([]models.FiscalPeriod, error) {
	c.mu.Lock()
	if c.periodSymbol == symbol {
		rows := append([]models.FiscalPeriod(nil), c.periodRows...)
		c.periodSymbol = ""
		c.periodRows = nil
		c.mu.Unlock()
		return rows, nil
	}
	c.mu.Unlock()
	if _, err := c.Financials(ctx, symbol); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.periodSymbol != symbol {
		return nil, providers.ErrUnavailable
	}
	rows := append([]models.FiscalPeriod(nil), c.periodRows...)
	c.periodSymbol = ""
	c.periodRows = nil
	return rows, nil
}

func fiscalEvidence(facts factsResponse, s submission) []models.FiscalPeriod {
	filings := map[string]models.FiscalPeriod{}
	r := s.Filings.Recent
	for i, a := range r.AccessionNumber {
		if i >= len(r.Form) || i >= len(r.ReportDate) || i >= len(r.FilingDate) {
			continue
		}
		form := r.Form[i]
		if form != "10-Q" && form != "10-Q/A" && form != "10-K" && form != "10-K/A" {
			continue
		}
		end, e := time.Parse("2006-01-02", r.ReportDate[i])
		if e != nil {
			continue
		}
		filed, e := time.Parse("2006-01-02", r.FilingDate[i])
		if e != nil {
			continue
		}
		filings[a] = models.FiscalPeriod{PeriodEnd: end, Filed: filed, Accession: a, Form: form}
	}
	// All concepts vote on FY/FP for their own filing period. Comparative ends
	// are excluded. Conflicting metadata is retained explicitly as ambiguous.
	byAcc := map[string]models.FiscalPeriod{}
	annual := map[string]bool{}
	for _, concept := range facts.Facts["us-gaap"] {
		for _, unit := range concept.Units {
			for _, f := range unit {
				p, ok := filings[f.Accn]
				if !ok || p.PeriodEnd.Format("2006-01-02") != f.End || f.FY < 1900 {
					continue
				}
				q := 0
				source := "sec_explicit"
				if (f.FP == "Q1" || f.FP == "Q2" || f.FP == "Q3") && strings.HasPrefix(p.Form, "10-Q") {
					q = int(f.FP[1] - '0')
				}
				if f.FP == "FY" && strings.HasPrefix(p.Form, "10-K") {
					q = 4
					source = "sec_annual_q4"
					start, e := time.Parse("2006-01-02", f.Start)
					days := p.PeriodEnd.Sub(start).Hours() / 24
					if e == nil && f.Val != nil && days >= 330 && days <= 400 {
						annual[f.Accn] = true
					}
				}
				if q == 0 {
					continue
				}
				p.FiscalYear = f.FY
				p.FiscalQuarter = q
				p.Source = source
				if old, ok := byAcc[f.Accn]; ok {
					p.Ambiguous = old.Ambiguous || old.FiscalYear != p.FiscalYear || old.FiscalQuarter != p.FiscalQuarter
				}
				byAcc[f.Accn] = p
			}
		}
	}
	byEnd := map[string]models.FiscalPeriod{}
	for a, p := range byAcc {
		if p.FiscalQuarter == 4 && !annual[a] {
			continue
		}
		key := p.PeriodEnd.Format("2006-01-02")
		if old, ok := byEnd[key]; ok {
			conflict := old.Ambiguous || p.Ambiguous || old.FiscalYear != p.FiscalYear || old.FiscalQuarter != p.FiscalQuarter
			if old.Filed.Before(p.Filed) {
				p = old
			}
			p.Ambiguous = conflict
		}
		byEnd[key] = p
	}
	// Retain unlabeled filed ends so surrounding explicit periods can support
	// a conservative sequence fallback even when Company Facts omits FP.
	for _, p := range filings {
		key := p.PeriodEnd.Format("2006-01-02")
		if old, ok := byEnd[key]; !ok || (old.FiscalQuarter == 0 && p.Filed.Before(old.Filed)) {
			p.Source = "financial_period_match"
			byEnd[key] = p
		}
	}
	// Reused FY/quarter across different ends is inconsistent source metadata.
	for k, p := range byEnd {
		for j, other := range byEnd {
			if p.FiscalQuarter != 0 && j != k && other.FiscalYear == p.FiscalYear && other.FiscalQuarter == p.FiscalQuarter {
				p.Ambiguous = true
			}
		}
		byEnd[k] = p
	}
	out := []models.FiscalPeriod{}
	for _, p := range byEnd {
		out = append(out, p)
	}
	return out
}
