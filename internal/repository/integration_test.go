//go:build integration

package repository

import (
	"context"
	"database/sql"
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
	calendar, err := s.Calendar(ctx, CalendarFilter{From: d, To: d.AddDate(0, 0, 1), Watchlist: true})
	if err != nil || len(calendar) != 1 {
		t.Fatal(calendar, err)
	}
	if err = s.RecordSync(ctx, "yahoo", "company:TEST", "success"); err != nil {
		t.Fatal(err)
	}
	before, err := s.SyncStates(ctx)
	if err != nil || len(before) != 1 || before[0].LastSuccessAt == nil {
		t.Fatal("missing initial success", err)
	}
	lastSuccess := *before[0].LastSuccessAt
	if err = s.RecordSync(ctx, "yahoo", "company:TEST", "error"); err != nil {
		t.Fatal(err)
	}
	states, err := s.SyncStates(ctx)
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
		states, err = s.SyncStates(ctx)
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
	reaction.Returns[1] = models.Ptr(0.0)
	for i := 0; i < 2; i++ {
		if err = s.UpsertReaction(ctx, reaction); err != nil {
			t.Fatal(err)
		}
	}
	rs, err := s.Reactions(ctx, first.ID)
	if err != nil || rs[events[0].ID].Returns[1] == nil {
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
	pool.Close()
	if err = goose.DownToContext(ctx, db, ".", 0); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpContext(ctx, db, "."); err != nil {
		t.Fatal(err)
	}
}
