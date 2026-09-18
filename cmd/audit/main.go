// Audit freezes a stratified sample and measures real database coverage.
package main

import (
	"context"
	"earnings-dashboard/internal/analytics"
	"earnings-dashboard/internal/config"
	"earnings-dashboard/internal/database"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/repository"
	"earnings-dashboard/internal/service"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if err := run(); err != nil {
		slog.Error("audit failed", "error", err)
		os.Exit(1)
	}
}
func run() error {
	sample := flag.String("sample", "docs/audits/sample.json", "frozen sample; created only if missing")
	output := flag.String("out", "docs/audits/coverage.json", "report path")
	refresh := flag.Bool("refresh", false, "refresh every sample company before measuring")
	financialsOnly := flag.Bool("refresh-financials", false, "refresh fundamentals for every sample company before measuring")
	flag.Parse()
	for _, path := range []string{*sample, *output} {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL, cfg.DatabaseTimeout)
	if err != nil {
		return err
	}
	defer db.Close()
	symbols := []string{}
	data, err := os.ReadFile(*sample)
	if os.IsNotExist(err) {
		rows, e := db.Query(ctx, `WITH candidates AS (
   SELECT c.symbol,c.market_cap,COALESCE(c.sector,'Unknown') sector,
   CASE WHEN c.market_cap>=200e9 THEN 'mega' WHEN c.market_cap>=10e9 THEN 'large' ELSE 'mid' END cap_group,
   EXISTS(SELECT 1 FROM earnings_events e WHERE e.company_id=c.id AND e.report_date BETWEEN (now() AT TIME ZONE 'America/New_York')::date-7 AND (now() AT TIME ZONE 'America/New_York')::date+14) reporting
   FROM companies c WHERE c.universe_eligible), ranked AS (
   SELECT *,row_number() OVER(PARTITION BY cap_group,sector ORDER BY reporting DESC,md5(symbol)) n FROM candidates)
   SELECT symbol FROM ranked ORDER BY n,reporting DESC,cap_group,sector,symbol LIMIT 60`)
		if e != nil {
			return e
		}
		for rows.Next() {
			var s string
			if e = rows.Scan(&s); e != nil {
				rows.Close()
				return e
			}
			symbols = append(symbols, s)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(symbols) < 50 {
			return fmt.Errorf("need at least 50 eligible companies, found %d", len(symbols))
		}
		data, _ = json.MarshalIndent(symbols, "", "  ")
		if err = os.WriteFile(*sample, data, 0644); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err = json.Unmarshal(data, &symbols); err != nil {
		return err
	}
	if len(symbols) < 50 {
		return fmt.Errorf("sample must contain at least 50 companies")
	}
	seen := map[string]bool{}
	for _, symbol := range symbols {
		if seen[symbol] {
			return fmt.Errorf("duplicate sample symbol: %s", symbol)
		}
		seen[symbol] = true
	}
	store := repository.New(db)
	syncer := service.New(store, cfg.SECUserAgent, slog.Default())
	failures := []string{}
	if *refresh || *financialsOnly {
		for i, s := range symbols {
			slog.Info("coverage refresh", "index", i+1, "total", len(symbols), "symbol", s)
			status := "success"
			fn := syncer.SyncAll
			if *financialsOnly {
				fn = syncer.SyncFinancials
			}
			if e := fn(ctx, s); e != nil {
				failures = append(failures, s)
				status = "error"
			}
			if e := service.Recalculate(ctx, store, s, time.Now()); e != nil {
				return e
			}
			if !*financialsOnly {
				if e := store.RecordSync(ctx, "yahoo", "all:"+s, status); e != nil {
					return e
				}
			}
		}
	}
	rows, err := db.Query(ctx, `SELECT c.symbol,COALESCE(c.sector,'Unknown'),c.market_cap,c.universe_eligible,c.fiscal_year_end,
 (c.name IS NOT NULL)::int,(c.market_cap IS NOT NULL)::int,(c.sector IS NOT NULL)::int,(c.industry IS NOT NULL)::int,(c.description IS NOT NULL)::int,
 (SELECT count(*) FROM earnings_events WHERE company_id=c.id AND report_date<current_date),
 (SELECT count(*) FROM earnings_events WHERE company_id=c.id AND eps_surprise_pct IS NOT NULL),
 (SELECT count(*) FROM canonical_quarterly_financials WHERE company_id=c.id AND revenue IS NOT NULL),
 (SELECT count(*) FROM canonical_quarterly_financials WHERE company_id=c.id AND diluted_eps IS NOT NULL),
 (SELECT count(*) FROM historical_prices WHERE company_id=c.id),
 (SELECT count(*) FROM earnings_reactions r JOIN earnings_events e ON e.id=r.earnings_event_id WHERE e.company_id=c.id AND r.opening_gap_pct IS NOT NULL),
 (SELECT count(*) FROM earnings_reactions r JOIN earnings_events e ON e.id=r.earnings_event_id WHERE e.company_id=c.id AND r.event_day_return_pct IS NOT NULL),
 (SELECT count(*) FROM earnings_reactions r JOIN earnings_events e ON e.id=r.earnings_event_id WHERE e.company_id=c.id AND r.return_1w IS NOT NULL),
 (SELECT count(*) FROM earnings_reactions r JOIN earnings_events e ON e.id=r.earnings_event_id WHERE e.company_id=c.id AND r.return_1m IS NOT NULL),
 (SELECT count(*) FROM earnings_events WHERE company_id=c.id AND fiscal_year IS NOT NULL AND fiscal_quarter IS NOT NULL),
 (SELECT count(*) FROM earnings_events WHERE company_id=c.id AND revenue_surprise_pct IS NOT NULL)
 FROM companies c WHERE symbol=ANY($1) ORDER BY symbol`, symbols)
	if err != nil {
		return err
	}
	defer rows.Close()
	fields := []string{"company_profile", "market_cap", "sector", "industry", "description", "earnings_events", "eps_surprise", "quarterly_revenue", "quarterly_eps", "daily_prices", "opening_gap", "event_day", "return_1w", "return_1m", "quarter_labels", "revenue_surprise"}
	records := []map[string]any{}
	covered := make([]int, len(fields))
	totals := make([]int, len(fields))
	for rows.Next() {
		var sym, sector string
		var cap *float64
		var eligible bool
		var fiscalYearEnd *string
		counts := make([]int, len(fields))
		targets := []any{&sym, &sector, &cap, &eligible, &fiscalYearEnd}
		for i := range counts {
			targets = append(targets, &counts[i])
		}
		if err = rows.Scan(targets...); err != nil {
			return err
		}
		r := map[string]any{"symbol": sym, "sector_name": sector, "market_cap_usd": cap, "eligible": eligible, "fiscal_year_end": fiscalYearEnd}
		for i, f := range fields {
			r[f] = counts[i]
			totals[i] += counts[i]
			if counts[i] > 0 {
				covered[i]++
			}
		}
		records = append(records, r)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if len(records) != len(symbols) {
		return fmt.Errorf("sample symbols missing from database")
	}
	rows.Close()
	history := []models.History{}
	for _, symbol := range symbols {
		c, e := store.Company(ctx, symbol)
		if e != nil {
			return e
		}
		events, e := store.Events(ctx, c.ID)
		if e != nil {
			return e
		}
		reactions, e := store.Reactions(ctx, c.ID)
		if e != nil {
			return e
		}
		for _, event := range events {
			history = append(history, models.History{Event: event, Reaction: reactions[event.ID]})
		}
	}
	aggregates := map[string]any{}
	for i, f := range fields {
		aggregates[f] = map[string]any{"companies_with_data": covered[i], "company_coverage_pct": float64(covered[i]) * 100 / float64(len(records)), "observations": totals[i]}
	}
	report := map[string]any{"generated_at": time.Now().UTC(), "sample_size": len(symbols), "definition": "Company coverage: at least one non-null observation per sampled company; counts do not imply complete historical coverage. Profile means a provider-sourced name. Frozen sample stratifies sector and market-cap bands, prioritizing current/next earnings.", "refresh_failures": failures, "aggregate": aggregates, "companies": records, "eps_behaviour": analytics.Behaviour(history, false), "gap_behaviour": analytics.Behaviour(history, true)}
	report["refresh_all"] = *refresh
	report["refresh_financials_only"] = *financialsOnly
	data, err = json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(*output, data, 0644)
}
