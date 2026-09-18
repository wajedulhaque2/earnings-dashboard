package repository

import "context"

// Link only exact same-day filing/earnings associations with one directly
// reported quarter. Never infer fiscal quarters from calendar months or proximity.
func (s *Store) LinkFiscalLabels(ctx context.Context, id int64) error {
	_, err := s.db.Exec(ctx, `WITH candidates AS (
 SELECT e.id,min(q.fiscal_year) fy,min(q.fiscal_quarter) fq,min(q.period_end) period_end
 FROM earnings_events e JOIN filings f ON f.company_id=e.company_id AND f.filed=e.report_date
 JOIN quarterly_financials q ON q.company_id=e.company_id AND q.period_end=f.report_date
 WHERE e.company_id=$1 AND f.form IN('10-Q','10-K') AND q.fiscal_year IS NOT NULL AND q.fiscal_quarter IS NOT NULL
 GROUP BY e.id HAVING count(DISTINCT q.period_end)=1)
 , desired AS (SELECT e.id,c.fy,c.fq,c.period_end FROM earnings_events e LEFT JOIN candidates c ON c.id=e.id WHERE e.company_id=$1 AND (e.fiscal_year IS NULL OR e.fiscal_label_source='sec_same_day'))
 UPDATE earnings_events e SET fiscal_year=c.fy,fiscal_quarter=c.fq,period_end=c.period_end,fiscal_label_source=CASE WHEN c.fy IS NOT NULL THEN 'sec_same_day' END
 FROM desired c WHERE e.id=c.id`, id)
	return err
}
