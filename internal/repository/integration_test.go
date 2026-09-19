//go:build integration

package repository

import (
	"context"
	"database/sql"
	"earnings-dashboard/internal/earnings"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/migrations"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"net/url"
	"os"
	"testing"
	"time"
)

// This test uses only an explicitly configured local disposable PostgreSQL database.
func TestPostgresUpsertsAndMigrations(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for local PostgreSQL integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("integration_%d", time.Now().UnixNano())
	if _, err = admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE") }()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	goose.SetBaseFS(migrations.Files)
	goose.SetLogger(goose.NopLogger())
	if err = goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = goose.UpContext(ctx, db, "."); err != nil {
			t.Fatal(err)
		}
	}
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := New(pool)
	if err = s.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	company := models.Company{Symbol: "TEST", Name: models.Text("Synthetic test fixture"), MarketCap: models.Ptr(100.0), Currency: models.Text("USD")}
	first, err := s.UpsertCompany(ctx, company)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.UpsertCompany(ctx, models.Company{Symbol: "TEST"})
	if err != nil || first.ID != second.ID {
		t.Fatal(err, first, second)
	}
	read, err := s.Company(ctx, "test")
	if err != nil || read.MarketCap == nil || *read.MarketCap != 100 || read.Name == nil {
		t.Fatal("lost company fields", err)
	}
	d := time.Date(2025, 3, 31, 0, 0, 0, 0, time.UTC)
	for _, f := range []models.Financial{{CompanyID: first.ID, PeriodEnd: d, Revenue: models.Ptr(100.0), Source: "sec", Currency: "USD"}, {CompanyID: first.ID, PeriodEnd: d, Revenue: models.Ptr(200.0), DilutedEPS: models.Ptr(2.0), Source: "yahoo", Currency: "USD"}, {CompanyID: first.ID, PeriodEnd: d, DilutedEPS: models.Ptr(3.0), Source: "yahoo", Currency: "USD"}} {
		if err = s.UpsertFinancial(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	fs, err := s.Financials(ctx, first.ID)
	if err != nil || len(fs) != 1 || *fs[0].Revenue != 100 || *fs[0].DilutedEPS != 3 || *fs[0].RevenueSource != "sec" || *fs[0].EPSSource != "yahoo" {
		t.Fatal("financial precedence", fs, err)
	}
	e := models.Event{CompanyID: first.ID, ReportDate: d, Session: "AMC", EPSActual: models.Ptr(0.0), Source: "yahoo"}
	if err = s.UpsertEvent(ctx, e); err != nil {
		t.Fatal(err)
	}
	e.EPSActual = nil
	e.Session = "UNKNOWN"
	if err = s.UpsertEvent(ctx, e); err != nil {
		t.Fatal(err)
	}
	events, err := s.Events(ctx, first.ID)
	if err != nil || len(events) != 1 || events[0].EPSActual == nil || events[0].Session != "AMC" {
		t.Fatal("event preservation", events, err)
	}
	p := models.Price{CompanyID: first.ID, Date: d, Close: models.Ptr(10.0), Source: "yahoo"}
	if err = s.UpsertPrice(ctx, p); err != nil {
		t.Fatal(err)
	}
	p.Close = nil
	if err = s.UpsertPrice(ctx, p); err != nil {
		t.Fatal(err)
	}
	prices, err := s.Prices(ctx, first.ID)
	if err != nil || len(prices) != 1 || *prices[0].Close != 10 {
		t.Fatal(prices, err)
	}
	for i := 0; i < 2; i++ {
		if err = s.SetWatchlist(ctx, "TEST", true); err != nil {
			t.Fatal(err)
		}
	}
	watch, err := s.Watchlist(ctx)
	if err != nil || len(watch) != 1 {
		t.Fatal(watch, err)
	}
	if _, err = s.db.Exec(ctx, "UPDATE companies SET universe_eligible=true WHERE symbol='TEST'"); err != nil {
		t.Fatal(err)
	}
	calendar, err := s.Calendar(ctx, CalendarFilter{From: d, To: d.AddDate(0, 0, 1), Watchlist: true})
	if err != nil || len(calendar) != 1 {
		t.Fatal(calendar, err)
	}
	if err = s.RecordSync(ctx, "yahoo", "company:TEST", "success"); err != nil {
		t.Fatal(err)
	}
	before, err := s.SyncStates(ctx, "all")
	if err != nil || len(before) != 1 || before[0].LastSuccessAt == nil {
		t.Fatal("missing initial success", err)
	}
	lastSuccess := *before[0].LastSuccessAt
	if err = s.RecordSync(ctx, "yahoo", "company:TEST", "error"); err != nil {
		t.Fatal(err)
	}
	states, err := s.SyncStates(ctx, "all")
	if err != nil || len(states) != 1 || states[0].LastSuccessAt == nil || states[0].LatestAttemptStatus != "error" {
		t.Fatal(states, err)
	}
	if !states[0].LastSuccessAt.Equal(lastSuccess) || states[0].LatestAttemptAt == nil || states[0].LatestAttemptAt.Before(lastSuccess) || states[0].Operation != "company" || states[0].Symbol != "TEST" {
		t.Fatal("attempt overwrote success or lost identity", states)
	}
	for _, status := range []string{"unsupported", "not_attempted", "success"} {
		if err = s.RecordSync(ctx, "yahoo", "company:TEST", status); err != nil {
			t.Fatal(err)
		}
		states, err = s.SyncStates(ctx, "all")
		if err != nil || states[0].LatestAttemptStatus != status || states[0].LatestErrorCategory != nil {
			t.Fatal(states, err)
		}
		if status != "success" && !states[0].LastSuccessAt.Equal(lastSuccess) {
			t.Fatal("lost last success")
		}
		if status == "not_attempted" && states[0].LatestAttemptAt != nil {
			t.Fatal("invented attempt timestamp")
		}
	}
	reaction := models.Reaction{EventID: events[0].ID, Methodology: "test"}
	reaction.Returns[0] = models.Ptr(0.05)
	reaction.Returns[1] = models.Ptr(0.0)
	for i := 0; i < 2; i++ {
		if err = s.UpsertReaction(ctx, reaction); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := s.Reactions(ctx, first.ID)
	if err != nil || rs[events[0].ID].Returns[1] == nil || rs[events[0].ID].Returns[0] == nil || *rs[events[0].ID].Returns[0] != .05 {
		t.Fatal(rs, err)
	}
	snapshot := models.Snapshot{CompanyID: first.ID, Time: d.Add(9 * time.Hour), Session: "PRE", Price: 10, Source: "yahoo"}
	for i := 0; i < 2; i++ {
		if err = s.UpsertSnapshot(ctx, snapshot); err != nil {
			t.Fatal(err)
		}
	}
	snaps, err := s.Snapshots(ctx, first.ID)
	if err != nil || len(snaps) != 1 {
		t.Fatal(snaps, err)
	}
	if err = s.Queue(ctx, "TEST"); err != nil {
		t.Fatal(err)
	}
	requests, err := s.Requests(ctx)
	if err != nil || len(requests) != 1 {
		t.Fatal(requests, err)
	}
	if err = s.FinishRequest(ctx, "TEST", true); err != nil {
		t.Fatal(err)
	}
	// Universe publication, default calendar filtering and automatic queueing
	// are verified against PostgreSQL rather than mocked SQL strings.
	outside, err := s.UpsertCompany(ctx, models.Company{Symbol: "OUTSIDE", Name: models.Text("Outside fixture")})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.UpsertEvent(ctx, models.Event{CompanyID: outside.ID, ReportDate: d, Session: "BMO", Source: "yahoo"}); err != nil {
		t.Fatal(err)
	}
	if err = s.ApplyUniverse(ctx, []models.Company{{Symbol: "TEST", UniverseEligible: true, UniverseReason: "eligible"}, {Symbol: "OUTSIDE", UniverseReason: "outside_supported_exchanges"}}); err != nil {
		t.Fatal(err)
	}
	merged, err := s.UpsertCompany(ctx, models.Company{Symbol: "TEST", Description: models.Text("Updated profile")})
	if err != nil || !merged.UniverseEligible {
		t.Fatal("profile refresh lost universe membership", err)
	}
	calendar, err = s.Calendar(ctx, CalendarFilter{From: d, To: d.AddDate(0, 0, 1)})
	if err != nil || len(calendar) != 1 || calendar[0].Company.Symbol != "TEST" {
		t.Fatal("outside company leaked into calendar", calendar, err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err = s.UpsertEvent(ctx, models.Event{CompanyID: first.ID, ReportDate: today, Session: "AMC", Source: "yahoo"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.QueueCalendar(ctx); err != nil {
			t.Fatal(err)
		}
	}
	requests, err = s.Requests(ctx)
	if err != nil || len(requests) != 1 || requests[0] != "TEST" {
		t.Fatal("calendar was not queued idempotently", requests, err)
	}
	if err = s.RecordSync(ctx, "sec", "financials:OUTSIDE", "unsupported"); err != nil {
		t.Fatal(err)
	}
	filtered, err := s.SyncStates(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range filtered {
		if state.LatestAttemptStatus == "unsupported" {
			t.Fatal("unsupported dominated overview")
		}
	}
	filtered, err = s.SyncStates(ctx, "unsupported")
	if err != nil || len(filtered) != 1 {
		t.Fatal(filtered, err)
	}
	if err = s.ApplyUniverse(ctx, nil); err == nil {
		t.Fatal("empty snapshot accepted")
	}
	merged, err = s.Company(ctx, "TEST")
	if err != nil || !merged.UniverseEligible {
		t.Fatal("empty screen erased universe")
	}
	quarterEnd := time.Date(2025, 6, 28, 0, 0, 0, 0, time.UTC)
	for _, f := range []models.Financial{
		{CompanyID: first.ID, PeriodEnd: quarterEnd, Source: "sec", Currency: "USD", Revenue: models.Ptr(150.0), DilutedEPS: models.Ptr(4.0)},
		{CompanyID: first.ID, PeriodEnd: quarterEnd.AddDate(0, 0, 2), Source: "yahoo", Currency: "USD", Revenue: models.Ptr(150.0), DilutedEPS: models.Ptr(4.0)},
	} {
		if err = s.UpsertFinancial(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	fs, err = s.Financials(ctx, first.ID)
	if err != nil || len(fs) != 2 {
		t.Fatal("duplicate source period shown twice", fs, err)
	}
	var rawCount int
	if err = s.db.QueryRow(ctx, "SELECT count(*) FROM quarterly_financials WHERE company_id=$1", first.ID).Scan(&rawCount); err != nil || rawCount != 3 {
		t.Fatal("raw observations lost", rawCount, err)
	}
	testFiscalEnrichment(t, s)
	testRevenueQuota(t, s)
	pool.Close()
	if err = goose.DownToContext(ctx, db, ".", 0); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpContext(ctx, db, "."); err != nil {
		t.Fatal(err)
	}
}

func testRevenueQuota(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 24; i++ {
		// Move only the spacing timestamp; keep all reservations in the real
		// rolling window so the daily limit is exercised without sleeping.
		if _, err := s.db.Exec(ctx, `UPDATE revenue_provider_budget SET last_request=now()-interval '14 seconds'`); err != nil {
			t.Fatal(err)
		}
		ok, err := s.ReserveRevenueRequest(ctx)
		if err != nil || !ok {
			t.Fatal(i, ok, err)
		}
		ok, err = s.ReserveRevenueRequest(ctx)
		if err != nil || ok {
			t.Fatal("minimum spacing not enforced", err)
		}
	}
	s.db.Exec(ctx, `UPDATE revenue_provider_budget SET last_request=now()-interval '14 seconds'`)
	ok, err := s.ReserveRevenueRequest(ctx)
	if err != nil || ok {
		t.Fatal("rolling quota exceeded", err)
	}
	if err = s.SaveRevenueCache(ctx, "TEST", "EARNINGS", []byte(`{"symbol":"TEST","quarterlyEarnings":[]}`)); err != nil {
		t.Fatal(err)
	}
	b, err := s.RevenueCache(ctx, "TEST", "EARNINGS")
	if err != nil || len(b) == 0 {
		t.Fatal("cache not persisted", err)
	}
	s.db.Exec(ctx, `UPDATE revenue_provider_cache SET fetched_at=now()-interval '8 days'`)
	b, err = s.RevenueCache(ctx, "TEST", "EARNINGS")
	if err != nil || len(b) != 0 {
		t.Fatal("expired cache reused", err)
	}
}

func testFiscalEnrichment(t *testing.T, s *Store) {
	t.Helper()
	ctx := context.Background()
	c, err := s.UpsertCompany(ctx, models.Company{Symbol: "FISCAL", Country: models.Text("United States"), Currency: models.Text("USD")})
	if err != nil {
		t.Fatal(err)
	}
	end := time.Date(2025, 4, 27, 0, 0, 0, 0, time.UTC)
	date := end.AddDate(0, 0, 30)
	e := models.Event{CompanyID: c.ID, Symbol: c.Symbol, ReportDate: date, Source: "yahoo"}
	if err = s.UpsertEvent(ctx, e); err != nil {
		t.Fatal(err)
	}
	p := models.FiscalPeriod{CompanyID: c.ID, PeriodEnd: end, Filed: date.AddDate(0, 0, 2), FiscalYear: 2026, FiscalQuarter: 1, Accession: "fixture", Form: "10-Q", Source: "sec_explicit"}
	if err = s.UpsertFiscalPeriod(ctx, p); err != nil {
		t.Fatal(err)
	}
	f := models.Financial{CompanyID: c.ID, PeriodEnd: end, Revenue: models.Ptr(110.0), Source: "sec", Currency: "USD"}
	if err = s.UpsertFinancial(ctx, f); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.LinkFiscalLabels(ctx, c.ID); err != nil {
			t.Fatal(err)
		}
	}
	read := func() models.Event {
		events, err := s.Events(ctx, c.ID)
		if err != nil || len(events) != 1 {
			t.Fatal(events, err)
		}
		return events[0]
	}
	e = read()
	if e.FiscalYear == nil || *e.FiscalYear != 2026 || e.PeriodEnd == nil || !e.PeriodEnd.Equal(end) || e.RevenueActual == nil || *e.RevenueActual != 110 || e.RevenueActualSource == nil || *e.RevenueActualSource != "sec" {
		t.Fatal("enrichment failed", e)
	}
	v := earnings.HistoricalRevenueEstimate{Symbol: c.Symbol, FiscalYear: 2026, FiscalQuarter: 1, PeriodEnd: end, ReportDate: date, ObservedAt: date.AddDate(0, 0, -1), Value: models.Ptr(100.0), Source: "yahoo", Currency: "USD", Historical: true}
	forward := v
	forward.Historical = false
	if err = s.StoreHistoricalRevenueEstimate(ctx, e, forward); err == nil {
		t.Fatal("forward consensus accepted")
	}
	if err = s.StoreHistoricalRevenueEstimate(ctx, e, v); err != nil {
		t.Fatal(err)
	}
	if err = s.LinkFiscalLabels(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	e = read()
	if e.RevenueSurprisePct == nil || *e.RevenueSurprisePct != .1 || e.RevenueEstimateSource == nil || *e.RevenueEstimateSource != "yahoo" {
		t.Fatal("fraction or independent provenance lost", e)
	}
	// A later Yahoo label/actual must not replace stronger SEC evidence.
	if err = s.UpsertEvent(ctx, models.Event{CompanyID: c.ID, ReportDate: date, FiscalYear: models.Ptr(2025), FiscalQuarter: models.Ptr(2), Source: "yahoo"}); err != nil {
		t.Fatal(err)
	}
	f.Source = "yahoo"
	f.Revenue = models.Ptr(999.0)
	if err = s.UpsertFinancial(ctx, f); err != nil {
		t.Fatal(err)
	}
	v.Value = models.Ptr(200.0)
	if err = s.StoreHistoricalRevenueEstimate(ctx, e, v); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = s.LinkFiscalLabels(ctx, c.ID); err != nil {
			t.Fatal(err)
		}
	}
	e = read()
	if *e.FiscalYear != 2026 || *e.FiscalQuarter != 1 || *e.RevenueActual != 110 || *e.RevenueEstimate != 100 || *e.RevenueSurprisePct != .1 {
		t.Fatal("strong evidence overwritten", e)
	}
	// Simulate incomplete weaker period evidence on a subsequent refresh.
	p.Source = "inferred_fiscal_sequence"
	p.FiscalYear = 2025
	p.FiscalQuarter = 2
	if err = s.UpsertFiscalPeriod(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err = s.LinkFiscalLabels(ctx, c.ID); err != nil {
		t.Fatal(err)
	}
	e = read()
	if *e.FiscalYear != 2026 || *e.FiscalQuarter != 1 {
		t.Fatal("weak mapping overwrote SEC", e)
	}
}
