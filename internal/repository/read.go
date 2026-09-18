package repository

import (
	"context"
	"earnings-dashboard/internal/models"
	"github.com/jackc/pgx/v5"
	"time"
)

const companyColumns = `id,symbol,name,exchange,country,sector,industry,description,logo_url,currency,timezone,cik,market_cap,latest_price,quote_time,updated_at,security_type,exchange_code,universe_eligible,universe_reason,universe_checked_at,fiscal_year_end`

func scanCompany(row pgx.Row) (models.Company, error) {
	var c models.Company
	err := row.Scan(&c.ID, &c.Symbol, &c.Name, &c.Exchange, &c.Country, &c.Sector, &c.Industry, &c.Description, &c.LogoURL, &c.Currency, &c.Timezone, &c.CIK, &c.MarketCap, &c.LatestPrice, &c.QuoteTime, &c.UpdatedAt, &c.SecurityType, &c.ExchangeCode, &c.UniverseEligible, &c.UniverseReason, &c.UniverseCheckedAt, &c.FiscalYearEnd)
	return c, err
}
func (s *Store) Company(ctx context.Context, symbol string) (models.Company, error) {
	sym, err := models.Symbol(symbol)
	if err != nil {
		return models.Company{}, err
	}
	return scanCompany(s.db.QueryRow(ctx, `SELECT `+companyColumns+` FROM companies WHERE symbol=$1`, sym))
}
func (s *Store) Search(ctx context.Context, q string) ([]models.Company, error) {
	rows, err := s.db.Query(ctx, `SELECT `+companyColumns+` FROM companies WHERE strpos(lower(symbol),lower($1))>0 OR strpos(lower(COALESCE(name,'')),lower($1))>0 ORDER BY (symbol=upper($1)) DESC,symbol LIMIT 50`, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Company{}
	for rows.Next() {
		c, e := scanCompany(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) Symbols(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT symbol FROM companies WHERE active ORDER BY symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

const eventColumns = `e.id,e.company_id,c.symbol,e.report_date,e.report_time,e.session,e.source,e.fiscal_year,e.fiscal_quarter,e.period_end,e.eps_estimate,e.eps_actual,e.eps_surprise,e.eps_surprise_pct,e.revenue_estimate,e.revenue_actual,e.revenue_surprise,e.revenue_surprise_pct`

func scanEvent(row pgx.Row) (models.Event, error) {
	var e models.Event
	err := row.Scan(&e.ID, &e.CompanyID, &e.Symbol, &e.ReportDate, &e.ReportTime, &e.Session, &e.Source, &e.FiscalYear, &e.FiscalQuarter, &e.PeriodEnd, &e.EPSEstimate, &e.EPSActual, &e.EPSSurprise, &e.EPSSurprisePct, &e.RevenueEstimate, &e.RevenueActual, &e.RevenueSurprise, &e.RevenueSurprisePct)
	return e, err
}
func (s *Store) Events(ctx context.Context, id int64) ([]models.Event, error) {
	rows, err := s.db.Query(ctx, `SELECT `+eventColumns+` FROM earnings_events e JOIN companies c ON c.id=e.company_id WHERE c.id=$1 ORDER BY e.report_date DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Event{}
	for rows.Next() {
		v, e := scanEvent(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type CalendarFilter struct {
	From, To               time.Time
	Session, Sector, Query string
	MinCap                 float64
	Watchlist              bool
}

func (s *Store) Calendar(ctx context.Context, f CalendarFilter) ([]models.CalendarItem, error) {
	rows, err := s.db.Query(ctx, `SELECT `+eventColumns+` FROM earnings_events e JOIN companies c ON c.id=e.company_id
 WHERE c.universe_eligible AND e.report_date >= $1 AND e.report_date < $2 AND ($3='' OR e.session=$3) AND ($4='' OR c.sector=$4)
 AND ($5='' OR strpos(lower(c.symbol),lower($5))>0 OR strpos(lower(COALESCE(c.name,'')),lower($5))>0)
 AND ($6::numeric=0 OR (c.currency='USD' AND c.market_cap>$6))
 AND (NOT $7 OR EXISTS(SELECT 1 FROM watchlist_companies wc JOIN watchlists w ON w.id=wc.watchlist_id WHERE wc.company_id=c.id AND w.name='Default'))
 ORDER BY e.report_date,c.symbol`, f.From, f.To, f.Session, f.Sector, f.Query, f.MinCap, f.Watchlist)
	if err != nil {
		return nil, err
	}
	events := []models.Event{}
	for rows.Next() {
		v, e := scanEvent(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		events = append(events, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []models.CalendarItem{}
	for _, e := range events {
		c, err := s.Company(ctx, e.Symbol)
		if err != nil {
			return nil, err
		}
		out = append(out, models.CalendarItem{Company: c, Event: e})
	}
	return out, nil
}
func (s *Store) Financials(ctx context.Context, id int64) ([]models.Financial, error) {
	rows, err := s.db.Query(ctx, `SELECT company_id,fiscal_year,fiscal_quarter,period_end,revenue,diluted_eps,source,COALESCE(currency,''),revenue_source,eps_source FROM canonical_quarterly_financials WHERE company_id=$1 ORDER BY period_end DESC LIMIT 12`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Financial{}
	for rows.Next() {
		var v models.Financial
		if err = rows.Scan(&v.CompanyID, &v.FiscalYear, &v.FiscalQuarter, &v.PeriodEnd, &v.Revenue, &v.DilutedEPS, &v.Source, &v.Currency, &v.RevenueSource, &v.EPSSource); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Prices(ctx context.Context, id int64) ([]models.Price, error) {
	rows, err := s.db.Query(ctx, `SELECT DISTINCT ON(trading_date) company_id,trading_date,open,high,low,close,adjusted_close,volume,source,split_ratio FROM historical_prices WHERE company_id=$1 ORDER BY trading_date,(source='yahoo') DESC`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Price{}
	for rows.Next() {
		var p models.Price
		if err = rows.Scan(&p.CompanyID, &p.Date, &p.Open, &p.High, &p.Low, &p.Close, &p.AdjustedClose, &p.Volume, &p.Source, &p.SplitRatio); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) Snapshots(ctx context.Context, id int64) ([]models.Snapshot, error) {
	rows, err := s.db.Query(ctx, `SELECT company_id,timestamp,session,price,source FROM intraday_snapshots WHERE company_id=$1 AND session='PRE' ORDER BY timestamp`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Snapshot{}
	for rows.Next() {
		var p models.Snapshot
		if err = rows.Scan(&p.CompanyID, &p.Time, &p.Session, &p.Price, &p.Source); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Store) Reactions(ctx context.Context, id int64) (map[int64]models.Reaction, error) {
	rows, err := s.db.Query(ctx, `SELECT r.earnings_event_id,r.previous_close,r.premarket_price,r.event_open,r.event_close,r.opening_gap_pct,r.event_day_return_pct,r.return_1d,r.return_2d,r.return_1w,r.return_2w,r.return_1m,r.return_3m,r.methodology_version,r.event_date FROM earnings_reactions r JOIN earnings_events e ON e.id=r.earnings_event_id WHERE e.company_id=$1`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]models.Reaction{}
	for rows.Next() {
		var v models.Reaction
		if err = rows.Scan(&v.EventID, &v.PreviousClose, &v.PremarketPrice, &v.EventOpen, &v.EventClose, &v.Returns[0], &v.Returns[1], &v.Returns[2], &v.Returns[3], &v.Returns[4], &v.Returns[5], &v.Returns[6], &v.Returns[7], &v.Methodology, &v.EventDate); err != nil {
			return nil, err
		}
		out[v.EventID] = v
	}
	return out, rows.Err()
}
func (s *Store) Watchlist(ctx context.Context) ([]models.Company, error) {
	rows, err := s.db.Query(ctx, `SELECT `+companyColumns+` FROM companies WHERE id IN (SELECT wc.company_id FROM watchlist_companies wc JOIN watchlists w ON w.id=wc.watchlist_id WHERE w.name='Default') ORDER BY symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Company{}
	for rows.Next() {
		c, e := scanCompany(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) SetWatchlist(ctx context.Context, symbol string, add bool) error {
	sym, err := models.Symbol(symbol)
	if err != nil {
		return err
	}
	if add {
		_, err = s.db.Exec(ctx, `INSERT INTO watchlist_companies(watchlist_id,company_id) SELECT w.id,c.id FROM watchlists w CROSS JOIN companies c WHERE w.name='Default' AND c.symbol=$1 ON CONFLICT DO NOTHING`, sym)
	} else {
		_, err = s.db.Exec(ctx, `DELETE FROM watchlist_companies WHERE company_id=(SELECT id FROM companies WHERE symbol=$1) AND watchlist_id=(SELECT id FROM watchlists WHERE name='Default')`, sym)
	}
	return err
}
func (s *Store) SyncStates(ctx context.Context, filters ...string) ([]models.SyncState, error) {
	filter := ""
	if len(filters) > 0 {
		filter = filters[0]
	}
	rows, err := s.db.Query(ctx, `SELECT provider,split_part(resource,':',1),split_part(resource,':',2),latest_attempt_at,latest_attempt_status,last_success_at,latest_error_category FROM provider_sync_state p LEFT JOIN companies c ON c.symbol=split_part(p.resource,':',2)
 WHERE CASE $1::text WHEN 'all' THEN true WHEN 'errors' THEN latest_attempt_status='error' WHEN 'unsupported' THEN latest_attempt_status='unsupported' WHEN 'universe' THEN COALESCE(c.universe_eligible,false) ELSE latest_attempt_status='error' OR (latest_attempt_status<>'unsupported' AND (COALESCE(c.universe_eligible,false) OR split_part(p.resource,':',2)='') AND latest_attempt_at>now()-interval '7 days') END
 ORDER BY CASE WHEN $1='' AND latest_attempt_status='error' THEN 0 ELSE 1 END,latest_attempt_at DESC NULLS LAST,provider,resource LIMIT 500`, filter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.SyncState{}
	for rows.Next() {
		var v models.SyncState
		if err = rows.Scan(&v.Provider, &v.Operation, &v.Symbol, &v.LatestAttemptAt, &v.LatestAttemptStatus, &v.LastSuccessAt, &v.LatestErrorCategory); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) Queue(ctx context.Context, symbol string) error {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `INSERT INTO sync_requests(symbol) VALUES($1) ON CONFLICT(symbol) DO UPDATE SET attempts=0,next_attempt=now(),requested_at=now()`, symbol)
	return err
}
func (s *Store) Requests(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT symbol FROM sync_requests WHERE next_attempt <= now() AND attempts<5 ORDER BY EXISTS(SELECT 1 FROM companies c JOIN earnings_events e ON e.company_id=c.id WHERE c.symbol=sync_requests.symbol AND c.universe_eligible AND e.report_date BETWEEN (now() AT TIME ZONE 'America/New_York')::date-7 AND (now() AT TIME ZONE 'America/New_York')::date+14) DESC, requested_at LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var v string
		if err = rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *Store) FinishRequest(ctx context.Context, symbol string, success bool) error {
	if success {
		_, err := s.db.Exec(ctx, `DELETE FROM sync_requests WHERE symbol=$1`, symbol)
		return err
	}
	_, err := s.db.Exec(ctx, `UPDATE sync_requests SET attempts=attempts+1,next_attempt=now()+interval '15 minutes' WHERE symbol=$1`, symbol)
	return err
}
