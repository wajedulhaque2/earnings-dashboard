package web

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/repository"
	"errors"
	"html"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type pageFixture struct {
	PageStore
	err error
}
type revenuePageFixture struct{ pageFixture }

func (revenuePageFixture) Events(context.Context, int64) ([]models.Event, error) {
	return []models.Event{{ID: 1, ReportDate: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC), FiscalYear: models.Ptr(2025), FiscalQuarter: models.Ptr(1), RevenueSurprisePct: models.Ptr(.1), EPSSurprisePct: models.Ptr(2.5)}}, nil
}
func TestRevenueFractionAndHelpText(t *testing.T) {
	r := NewRouter(readinessFunc(func(context.Context) error { return nil }), time.Second, slog.Default(), &App{Store: revenuePageFixture{}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/stocks/TEST", nil))
	body := html.UnescapeString(w.Body.String())
	for _, want := range []string{"+10.00%", "+2.50%", "2025 Q1", "Revenue surprise requires a historical analyst consensus estimate. Some older quarters are unavailable from free sources."} {
		if !strings.Contains(body, want) {
			t.Fatal("missing rendered value/help", want)
		}
	}
	if quarter(models.Event{PeriodEnd: models.Ptr(time.Now())}) != "N/A" {
		t.Fatal("period date presented as a known quarter")
	}
}

func (f pageFixture) Company(context.Context, string) (models.Company, error) {
	return models.Company{ID: 1, Symbol: "TEST", Name: models.Text("<script>alert(1)</script>")}, f.err
}
func (f pageFixture) Calendar(context.Context, repository.CalendarFilter) ([]models.CalendarItem, error) {
	return nil, f.err
}
func (f pageFixture) Events(context.Context, int64) ([]models.Event, error) { return nil, f.err }
func (f pageFixture) Financials(context.Context, int64) ([]models.Financial, error) {
	return nil, f.err
}
func (f pageFixture) Reactions(context.Context, int64) (map[int64]models.Reaction, error) {
	return nil, f.err
}
func (f pageFixture) Search(context.Context, string) ([]models.Company, error) { return nil, f.err }
func (f pageFixture) Watchlist(context.Context) ([]models.Company, error)      { return nil, f.err }
func (f pageFixture) SyncStates(context.Context, ...string) ([]models.SyncState, error) {
	return nil, f.err
}
func TestApplicationPages(t *testing.T) {
	r := NewRouter(readinessFunc(func(context.Context) error { return nil }), time.Second, slog.Default(), &App{Store: pageFixture{}})
	for _, path := range []string{"/calendar", "/stocks/TEST", "/search?q=TEST", "/watchlist", "/status"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "<script>alert(1)") {
			t.Fatal("unescaped company name")
		}
	}
}
func TestDatabaseFailureAndInvalidFilters(t *testing.T) {
	r := NewRouter(readinessFunc(func(context.Context) error { return nil }), time.Second, slog.Default(), &App{Store: pageFixture{err: errors.New("secret password")}})
	for _, tc := range []struct {
		path   string
		status int
	}{{"/calendar", 503}, {"/calendar?week=bad", 400}, {"/calendar?cap=NaN", 400}, {"/stocks/TEST", 503}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
	}
}
func TestMutationOrigin(t *testing.T) {
	r := NewRouter(readinessFunc(func(context.Context) error { return nil }), time.Second, slog.Default(), &App{Store: pageFixture{}})
	req := httptest.NewRequest("POST", "/watchlist", strings.NewReader("symbol=TEST&action=add"))
	req.Header.Set("Origin", "https://attacker.invalid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
