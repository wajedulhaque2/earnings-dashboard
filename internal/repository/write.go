package repository

import (
	"context"
	"earnings-dashboard/internal/models"
	"errors"
	"time"
)

func (s *Store) UpsertCompany(ctx context.Context, c models.Company) (models.Company, error) {
	sym, err := models.Symbol(c.Symbol)
	if err != nil {
		return c, err
	}
	c.Symbol = sym
	err = s.db.QueryRow(ctx, `INSERT INTO companies(symbol,name,exchange,country,sector,industry,description,logo_url,market_cap,currency,latest_price,quote_time,timezone,cik)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
 ON CONFLICT(symbol) DO UPDATE SET
 name=COALESCE(EXCLUDED.name,companies.name),exchange=COALESCE(EXCLUDED.exchange,companies.exchange),
 country=COALESCE(EXCLUDED.country,companies.country),sector=COALESCE(EXCLUDED.sector,companies.sector),
 industry=COALESCE(EXCLUDED.industry,companies.industry),description=COALESCE(EXCLUDED.description,companies.description),
 logo_url=COALESCE(EXCLUDED.logo_url,companies.logo_url),market_cap=COALESCE(EXCLUDED.market_cap,companies.market_cap),
 currency=COALESCE(EXCLUDED.currency,companies.currency),timezone=COALESCE(EXCLUDED.timezone,companies.timezone),cik=COALESCE(EXCLUDED.cik,companies.cik),
 latest_price=CASE WHEN EXCLUDED.quote_time >= companies.quote_time OR companies.quote_time IS NULL THEN COALESCE(EXCLUDED.latest_price,companies.latest_price) ELSE companies.latest_price END,
 quote_time=GREATEST(EXCLUDED.quote_time,companies.quote_time),updated_at=now() RETURNING id`, sym, c.Name, c.Exchange, c.Country, c.Sector, c.Industry, c.Description, c.LogoURL, c.MarketCap, c.Currency, c.LatestPrice, c.QuoteTime, c.Timezone, c.CIK).Scan(&c.ID)
	return c, err
}
func (s *Store) UpsertEvent(ctx context.Context, e models.Event) error {
	if e.Session == "" {
		e.Session = "UNKNOWN"
	}
	_, err := s.db.Exec(ctx, `INSERT INTO earnings_events(company_id,report_date,report_time,session,fiscal_year,fiscal_quarter,period_end,eps_estimate,eps_actual,eps_surprise,eps_surprise_pct,revenue_estimate,revenue_actual,revenue_surprise,revenue_surprise_pct,source)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
 ON CONFLICT(company_id,report_date) DO UPDATE SET
 report_time=COALESCE(EXCLUDED.report_time,earnings_events.report_time),session=CASE WHEN EXCLUDED.session='UNKNOWN' THEN earnings_events.session ELSE EXCLUDED.session END,
 fiscal_year=COALESCE(EXCLUDED.fiscal_year,earnings_events.fiscal_year),fiscal_quarter=COALESCE(EXCLUDED.fiscal_quarter,earnings_events.fiscal_quarter),period_end=COALESCE(EXCLUDED.period_end,earnings_events.period_end),
 eps_estimate=COALESCE(EXCLUDED.eps_estimate,earnings_events.eps_estimate),eps_actual=COALESCE(EXCLUDED.eps_actual,earnings_events.eps_actual),eps_surprise=COALESCE(EXCLUDED.eps_surprise,earnings_events.eps_surprise),eps_surprise_pct=COALESCE(EXCLUDED.eps_surprise_pct,earnings_events.eps_surprise_pct),
 revenue_estimate=COALESCE(EXCLUDED.revenue_estimate,earnings_events.revenue_estimate),revenue_actual=COALESCE(EXCLUDED.revenue_actual,earnings_events.revenue_actual),revenue_surprise=COALESCE(EXCLUDED.revenue_surprise,earnings_events.revenue_surprise),revenue_surprise_pct=COALESCE(EXCLUDED.revenue_surprise_pct,earnings_events.revenue_surprise_pct),source=EXCLUDED.source,updated_at=now()`, e.CompanyID, e.ReportDate, e.ReportTime, e.Session, e.FiscalYear, e.FiscalQuarter, e.PeriodEnd, e.EPSEstimate, e.EPSActual, e.EPSSurprise, e.EPSSurprisePct, e.RevenueEstimate, e.RevenueActual, e.RevenueSurprise, e.RevenueSurprisePct, e.Source)
	return err
}
func (s *Store) UpsertFinancial(ctx context.Context, f models.Financial) error {
	_, err := s.db.Exec(ctx, `INSERT INTO quarterly_financials(company_id,fiscal_year,fiscal_quarter,period_end,revenue,diluted_eps,source,currency,revenue_source,eps_source)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,CASE WHEN $5::numeric IS NOT NULL THEN $7 END,CASE WHEN $6::numeric IS NOT NULL THEN $7 END) ON CONFLICT(company_id,period_end) DO UPDATE SET
 revenue=CASE WHEN quarterly_financials.revenue_source='sec' AND EXCLUDED.source<>'sec' THEN quarterly_financials.revenue ELSE COALESCE(EXCLUDED.revenue,quarterly_financials.revenue) END,
 diluted_eps=CASE WHEN quarterly_financials.eps_source='sec' AND EXCLUDED.source<>'sec' THEN quarterly_financials.diluted_eps ELSE COALESCE(EXCLUDED.diluted_eps,quarterly_financials.diluted_eps) END,
 revenue_source=CASE WHEN quarterly_financials.revenue_source='sec' THEN 'sec' ELSE COALESCE(EXCLUDED.revenue_source,quarterly_financials.revenue_source) END,
 eps_source=CASE WHEN quarterly_financials.eps_source='sec' THEN 'sec' ELSE COALESCE(EXCLUDED.eps_source,quarterly_financials.eps_source) END,
 fiscal_year=COALESCE(EXCLUDED.fiscal_year,quarterly_financials.fiscal_year),fiscal_quarter=COALESCE(EXCLUDED.fiscal_quarter,quarterly_financials.fiscal_quarter),
 currency=COALESCE(quarterly_financials.currency,EXCLUDED.currency),source=CASE WHEN quarterly_financials.source='sec' THEN 'sec' ELSE EXCLUDED.source END,updated_at=now()
 WHERE quarterly_financials.currency IS NULL OR EXCLUDED.currency IS NULL OR quarterly_financials.currency=EXCLUDED.currency`, f.CompanyID, f.FiscalYear, f.FiscalQuarter, f.PeriodEnd, f.Revenue, f.DilutedEPS, f.Source, models.Text(f.Currency))
	return err
}
func (s *Store) UpsertPrice(ctx context.Context, p models.Price) error {
	_, err := s.db.Exec(ctx, `INSERT INTO historical_prices(company_id,trading_date,open,high,low,close,adjusted_close,volume,source,split_ratio) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
 ON CONFLICT(company_id,trading_date,source) DO UPDATE SET open=COALESCE(EXCLUDED.open,historical_prices.open),high=COALESCE(EXCLUDED.high,historical_prices.high),low=COALESCE(EXCLUDED.low,historical_prices.low),close=COALESCE(EXCLUDED.close,historical_prices.close),adjusted_close=COALESCE(EXCLUDED.adjusted_close,historical_prices.adjusted_close),volume=COALESCE(EXCLUDED.volume,historical_prices.volume),split_ratio=COALESCE(EXCLUDED.split_ratio,historical_prices.split_ratio),updated_at=now()`, p.CompanyID, p.Date, p.Open, p.High, p.Low, p.Close, p.AdjustedClose, p.Volume, p.Source, p.SplitRatio)
	return err
}
func (s *Store) UpsertSnapshot(ctx context.Context, p models.Snapshot) error {
	_, err := s.db.Exec(ctx, `INSERT INTO intraday_snapshots(company_id,timestamp,session,price,source) VALUES($1,$2,$3,$4,$5) ON CONFLICT(company_id,timestamp,source) DO UPDATE SET price=EXCLUDED.price,session=EXCLUDED.session`, p.CompanyID, p.Time, p.Session, p.Price, p.Source)
	return err
}
func (s *Store) UpsertFiling(ctx context.Context, f models.Filing) error {
	var report *time.Time
	if !f.ReportDate.IsZero() {
		report = &f.ReportDate
	}
	_, err := s.db.Exec(ctx, `INSERT INTO filings(company_id,accession,form,filed,report_date,document) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(company_id,accession) DO UPDATE SET form=EXCLUDED.form,filed=EXCLUDED.filed,report_date=COALESCE(EXCLUDED.report_date,filings.report_date),document=EXCLUDED.document`, f.CompanyID, f.Accession, f.Form, f.Filed, report, f.Document)
	return err
}
func (s *Store) RecordSync(ctx context.Context, provider, resource, status string) error {
	var reason *string
	if status == "error" {
		reason = models.Text("refresh_failed")
	}
	_, err := s.db.Exec(ctx, `INSERT INTO provider_sync_state(provider,resource,last_sync,status,error) VALUES($1,$2,CASE WHEN $3='success' THEN now() END,$3,$4) ON CONFLICT(provider,resource) DO UPDATE SET last_sync=COALESCE(EXCLUDED.last_sync,provider_sync_state.last_sync),status=EXCLUDED.status,error=EXCLUDED.error,attempted_at=now()`, provider, resource, status, reason)
	return err
}
func (s *Store) UpsertReaction(ctx context.Context, r models.Reaction) error {
	if r.Methodology == "" {
		return errors.New("reaction methodology required")
	}
	_, err := s.db.Exec(ctx, `INSERT INTO earnings_reactions(earnings_event_id,previous_close,premarket_price,event_open,event_close,premarket_return_pct,event_day_return_pct,return_1d,return_2d,return_1w,return_2w,return_1m,return_3m,methodology_version,event_date)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT(earnings_event_id) DO UPDATE SET
 previous_close=EXCLUDED.previous_close,premarket_price=EXCLUDED.premarket_price,event_open=EXCLUDED.event_open,event_close=EXCLUDED.event_close,
 premarket_return_pct=EXCLUDED.premarket_return_pct,event_day_return_pct=EXCLUDED.event_day_return_pct,return_1d=EXCLUDED.return_1d,return_2d=EXCLUDED.return_2d,return_1w=EXCLUDED.return_1w,return_2w=EXCLUDED.return_2w,return_1m=EXCLUDED.return_1m,return_3m=EXCLUDED.return_3m,methodology_version=EXCLUDED.methodology_version,event_date=EXCLUDED.event_date,calculated_at=now()`, r.EventID, r.PreviousClose, r.PremarketPrice, r.EventOpen, r.EventClose, r.Returns[0], r.Returns[1], r.Returns[2], r.Returns[3], r.Returns[4], r.Returns[5], r.Returns[6], r.Returns[7], r.Methodology, r.EventDate)
	return err
}
