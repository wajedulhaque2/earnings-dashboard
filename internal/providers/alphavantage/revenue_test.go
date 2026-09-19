package alphavantage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const estimateFixture = `{"symbol":"TEST","estimates":[
 {"date":"2025-03-31","horizon":"fiscal quarter","revenue_estimate_average":"100","eps_estimate_average":"1.2341"},
 {"date":"2025-12-31","horizon":"fiscal year","revenue_estimate_average":"400","eps_estimate_average":"5"},
 {"date":"2099-03-31","horizon":"fiscal quarter","revenue_estimate_average":"999","eps_estimate_average":"2"},
 {"date":"2024-12-31","horizon":"fiscal quarter","revenue_estimate_average":"90","eps_estimate_average":"1.10"}]}`
const historyFixture = `{"symbol":"TEST","quarterlyEarnings":[
 {"fiscalDateEnding":"2025-03-31","reportedDate":"2025-05-01","estimatedEPS":"1.23","reportedEPS":"1.30"},
 {"fiscalDateEnding":"2024-12-31","reportedDate":"2025-02-01","estimatedEPS":"1.20","reportedEPS":"1.30"}]}`

func TestHistoricalReportedQuarterOnly(t *testing.T) {
	rows, err := parseHistorical([]byte(estimateFixture), []byte(historyFixture), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil || len(rows) != 1 || rows[0].Value != 100 || rows[0].Source != "alphavantage" || rows[0].PeriodEnd.Format("2006-01-02") != "2025-03-31" {
		t.Fatal(rows, err)
	}
	// Forward, annual and incompatible historical EPS consensus were excluded.
	rows, err = parseHistorical([]byte(estimateFixture), []byte(historyFixture), time.Date(2025, 4, 30, 0, 0, 0, 0, time.UTC))
	if err != nil || len(rows) != 0 {
		t.Fatal("unreported quarter accepted", rows, err)
	}
}

type cacheFixture struct {
	data         map[string][]byte
	allowed      bool
	reservations int
}

func (c *cacheFixture) RevenueCache(_ context.Context, symbol, operation string) ([]byte, error) {
	return c.data[symbol+operation], nil
}
func (c *cacheFixture) SaveRevenueCache(_ context.Context, symbol, operation string, b []byte) error {
	c.data[symbol+operation] = b
	return nil
}
func (c *cacheFixture) ReserveRevenueRequest(context.Context) (bool, error) {
	c.reservations++
	return c.allowed, nil
}

func TestFreeRequestsCachedAndBudgeted(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch r.URL.Query().Get("function") {
		case "EARNINGS_ESTIMATES":
			w.Write([]byte(estimateFixture))
		case "EARNINGS":
			w.Write([]byte(historyFixture))
		default:
			t.Error("unexpected endpoint")
			http.Error(w, "invalid", 400)
		}
	}))
	defer srv.Close()
	cache := &cacheFixture{data: map[string][]byte{}, allowed: true}
	c := New("test-key", cache)
	c.Base = srv.URL
	c.Interval = 0
	for i := 0; i < 2; i++ {
		rows, err := c.HistoricalRevenue(context.Background(), "TEST")
		if err != nil || len(rows) != 1 {
			t.Fatal(rows, err)
		}
	}
	if calls != 2 || cache.reservations != 2 {
		t.Fatal("cache did not protect quota", calls, cache.reservations)
	}
	cache.allowed = false
	_, err := c.HistoricalRevenue(context.Background(), "OTHER")
	if !errors.Is(err, ErrBudget) || calls != 2 {
		t.Fatal("quota bypass", calls, err)
	}
}

func TestProviderErrorsNeverExposeKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"Information":"not available"}`)) }))
	defer srv.Close()
	cache := &cacheFixture{data: map[string][]byte{}, allowed: true}
	c := New("secret-key", cache)
	c.Base = srv.URL
	c.Interval = 0
	_, err := c.HistoricalRevenue(context.Background(), "TEST")
	if err == nil || len(cache.data) != 0 {
		t.Fatal("error response cached", err)
	}
	if err.Error() != "provider data unavailable" {
		t.Fatal("unsafe provider error", err)
	}
}
