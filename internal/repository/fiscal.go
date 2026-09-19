package repository

import (
	"context"
	"earnings-dashboard/internal/analytics"
	"earnings-dashboard/internal/earnings"
	"earnings-dashboard/internal/models"
	"errors"
	"time"
)

func (s *Store) UpsertFiscalPeriod(ctx context.Context, p models.FiscalPeriod) error {
	_, err := s.db.Exec(ctx, `INSERT INTO fiscal_periods(company_id,period_end,fiscal_year,fiscal_quarter,filed,accession,form,mapping_source,ambiguous)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(company_id,period_end) DO UPDATE SET fiscal_year=EXCLUDED.fiscal_year,fiscal_quarter=EXCLUDED.fiscal_quarter,filed=EXCLUDED.filed,accession=EXCLUDED.accession,form=EXCLUDED.form,mapping_source=EXCLUDED.mapping_source,ambiguous=EXCLUDED.ambiguous
 WHERE EXCLUDED.ambiguous OR EXCLUDED.mapping_source IN('sec_explicit','sec_annual_q4') OR fiscal_periods.mapping_source NOT IN('sec_explicit','sec_annual_q4')`, p.CompanyID, p.PeriodEnd, p.FiscalYear, p.FiscalQuarter, p.Filed, p.Accession, p.Form, p.Source, p.Ambiguous)
	return err
}

// LinkFiscalLabels is entirely database-backed and safe to repeat after any
// ingestion order. No numeric observation is inferred from fiscal sequence.
func (s *Store) LinkFiscalLabels(ctx context.Context, id int64) error {
	rows, err := s.db.Query(ctx, `SELECT period_end,fiscal_year,fiscal_quarter,filed,accession,form,mapping_source,ambiguous FROM fiscal_periods WHERE company_id=$1
 UNION ALL SELECT q.period_end,COALESCE(q.fiscal_year,0),COALESCE(q.fiscal_quarter,0),COALESCE((SELECT min(f.filed) FROM filings f WHERE f.company_id=q.company_id AND f.report_date=q.period_end AND f.form IN('10-Q','10-Q/A','10-K','10-K/A')),'0001-01-01'::date),'','','financial_period_match',false
 FROM canonical_quarterly_financials q WHERE q.company_id=$1 AND NOT EXISTS(SELECT 1 FROM fiscal_periods p WHERE p.company_id=q.company_id AND abs(p.period_end-q.period_end)<=7)`, id)
	if err != nil {
		return err
	}
	periods := []models.FiscalPeriod{}
	for rows.Next() {
		var p models.FiscalPeriod
		if err = rows.Scan(&p.PeriodEnd, &p.FiscalYear, &p.FiscalQuarter, &p.Filed, &p.Accession, &p.Form, &p.Source, &p.Ambiguous); err != nil {
			rows.Close()
			return err
		}
		periods = append(periods, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	periods = earnings.CompleteSequence(periods)
	events, err := s.Events(ctx, id)
	if err != nil {
		return err
	}
	var country, cik *string
	if err = s.db.QueryRow(ctx, `SELECT country,cik FROM companies WHERE id=$1`, id).Scan(&country, &cik); err != nil {
		return err
	}
	for _, e := range events {
		original := e
		loc, _ := time.LoadLocation("America/New_York")
		if !e.ReportDate.Before(earnings.Date(time.Now().In(loc))) {
			continue
		}
		// Withhold an SEC-derived label when that same filed period now has
		// conflicting explicit evidence. This is not a weaker inferred replacement.
		if e.FiscalLabelSource != nil && (*e.FiscalLabelSource == "sec_same_day" || *e.FiscalLabelSource == "sec_explicit" || *e.FiscalLabelSource == "sec_annual_q4") && e.PeriodEnd != nil {
			for _, p := range periods {
				if p.PeriodEnd.Equal(*e.PeriodEnd) && p.Ambiguous {
					e.FiscalYear = nil
					e.FiscalQuarter = nil
					e.FiscalLabelSource = nil
					break
				}
			}
		}
		matchEvent := e
		if e.FiscalLabelSource != nil && earnings.MappingStrength(*e.FiscalLabelSource) <= 2 {
			// Re-evaluate inferred/financial matches as newer fiscal evidence
			// arrives; the stored weak label must not lock us onto an older quarter.
			matchEvent.FiscalYear = nil
			matchEvent.FiscalQuarter = nil
			matchEvent.PeriodEnd = nil
			matchEvent.FiscalLabelSource = nil
		}
		p, reason := earnings.MatchFiscal(matchEvent, periods)
		if p != nil {
			source := ""
			if e.FiscalLabelSource != nil {
				source = *e.FiscalLabelSource
			}
			if earnings.MappingStrength(p.Source) >= earnings.MappingStrength(source) || e.FiscalYear == nil || e.FiscalQuarter == nil {
				e.FiscalYear = models.Ptr(p.FiscalYear)
				e.FiscalQuarter = models.Ptr(p.FiscalQuarter)
				e.FiscalLabelSource = models.Text(p.Source)
				e.PeriodEnd = models.Ptr(p.PeriodEnd)
			}
			if e.PeriodEnd == nil {
				e.PeriodEnd = models.Ptr(p.PeriodEnd)
			}
		} else if len(periods) == 0 || (country != nil && *country != "United States" && reason == "no_matching_filing") {
			reason = "insufficient_history"
			if cik == nil {
				reason = "no_sec_support"
			}
			if country != nil && *country != "United States" {
				reason = "foreign_issuer"
			}
		}
		if e.FiscalYear != nil && e.FiscalQuarter != nil {
			reason = ""
		}
		_, err = s.db.Exec(ctx, `UPDATE earnings_events SET fiscal_year=$2,fiscal_quarter=$3,period_end=$4,fiscal_label_source=$5,fiscal_mapping_reason=$6,
 revenue_actual=CASE WHEN period_end IS DISTINCT FROM $4 THEN NULL ELSE revenue_actual END,
 revenue_actual_source=CASE WHEN period_end IS DISTINCT FROM $4 THEN NULL ELSE revenue_actual_source END,
 revenue_estimate=CASE WHEN period_end IS DISTINCT FROM $4 THEN NULL ELSE revenue_estimate END,
 revenue_estimate_source=CASE WHEN period_end IS DISTINCT FROM $4 THEN NULL ELSE revenue_estimate_source END,
 revenue_surprise=CASE WHEN period_end IS DISTINCT FROM $4 THEN NULL ELSE revenue_surprise END,
 revenue_surprise_pct=CASE WHEN period_end IS DISTINCT FROM $4 THEN NULL ELSE revenue_surprise_pct END
 WHERE id=$1 AND fiscal_label_source IS NOT DISTINCT FROM $7 AND fiscal_year IS NOT DISTINCT FROM $8 AND fiscal_quarter IS NOT DISTINCT FROM $9 AND period_end IS NOT DISTINCT FROM $10`, e.ID, e.FiscalYear, e.FiscalQuarter, e.PeriodEnd, e.FiscalLabelSource, models.Text(reason), original.FiscalLabelSource, original.FiscalYear, original.FiscalQuarter, original.PeriodEnd)
		if err != nil {
			return err
		}
	}
	// Exact ends first. A Yahoo month-end alias is accepted only with matching
	// explicit FY/Q labels, within seven days, and exactly one matching record.
	// Annual financial amounts never enter canonical_quarterly_financials.
	_, err = s.db.Exec(ctx, `WITH chosen AS (
 SELECT e.id,q.revenue,q.revenue_source FROM earnings_events e JOIN LATERAL (
  SELECT q.revenue,q.revenue_source FROM canonical_quarterly_financials q
  WHERE q.company_id=e.company_id AND q.revenue IS NOT NULL AND q.currency='USD'
  AND (q.period_end=e.period_end OR (q.source='yahoo' AND q.fiscal_year=e.fiscal_year AND q.fiscal_quarter=e.fiscal_quarter AND abs(q.period_end-e.period_end)<=7
   AND (SELECT count(*) FROM canonical_quarterly_financials other WHERE other.company_id=e.company_id AND other.fiscal_year=e.fiscal_year AND other.fiscal_quarter=e.fiscal_quarter AND abs(other.period_end-e.period_end)<=7 AND other.revenue IS NOT NULL)=1))
  ORDER BY (q.revenue_source='sec') DESC,(q.period_end=e.period_end) DESC LIMIT 1
 )q ON true WHERE e.company_id=$1)
 UPDATE earnings_events e SET revenue_actual=q.revenue,revenue_actual_source=q.revenue_source FROM chosen q WHERE e.id=q.id AND (e.revenue_actual IS NULL OR (q.revenue_source='sec' AND e.revenue_actual_source IS DISTINCT FROM 'sec' AND e.revenue_surprise_pct IS NULL))`, id)
	if err != nil {
		return err
	}
	events, err = s.Events(ctx, id)
	if err != nil {
		return err
	}
	for _, e := range events {
		if e.RevenueActualSource == nil || e.RevenueEstimateSource == nil || e.RevenueSurprisePct != nil {
			continue
		}
		surprise := analytics.RevenueSurprise(e.RevenueActual, e.RevenueEstimate)
		if surprise == nil {
			continue
		}
		_, err = s.db.Exec(ctx, `UPDATE earnings_events SET revenue_surprise_pct=$2,revenue_surprise=revenue_actual-revenue_estimate WHERE id=$1 AND revenue_surprise_pct IS NULL`, e.ID, surprise)
		if err != nil {
			return err
		}
	}
	return nil
}

// StoreHistoricalRevenueEstimate accepts only a fully identified historical
// consensus observation; current forecast snapshots cannot enter this path.
func (s *Store) StoreHistoricalRevenueEstimate(ctx context.Context, event models.Event, v earnings.HistoricalRevenueEstimate) error {
	if !earnings.MatchRevenueEstimate(event, v) || v.Currency != "USD" {
		return errors.New("historical revenue estimate does not match event")
	}
	_, err := s.db.Exec(ctx, `UPDATE earnings_events e SET revenue_estimate=$2,revenue_estimate_source=$3 WHERE id=$1 AND revenue_estimate IS NULL AND fiscal_year=$4 AND fiscal_quarter=$5 AND period_end=$6 AND report_date=$7 AND company_id=(SELECT id FROM companies WHERE symbol=$8)`, event.ID, v.Value, v.Source, v.FiscalYear, v.FiscalQuarter, v.PeriodEnd, v.ReportDate, v.Symbol)
	return err
}
