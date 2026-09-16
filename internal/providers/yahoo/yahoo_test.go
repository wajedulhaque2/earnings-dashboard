package yahoo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// All numeric observations in these tests are synthetic parser fixtures, not seed data.
func fixture(t *testing.T, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.Base = srv.URL
	c.HTTP.Interval = 0
	return c
}
func TestSearchAndMissingValues(t *testing.T) {
	c := fixture(t, `{"quotes":[{"symbol":"TEST","longname":"Fixture Corp"},{"symbol":"../bad"}]}`)
	rows, err := c.Search(context.Background(), "fixture")
	if err != nil || len(rows) != 1 || rows[0].MarketCap != nil {
		t.Fatalf("%v %v", rows, err)
	}
}
func TestMalformedAndEmpty(t *testing.T) {
	for _, body := range []string{`{broken`, `{"chart":{"result":[],"error":null}}`} {
		c := fixture(t, body)
		if _, err := c.Prices(context.Background(), "TEST"); err == nil {
			t.Fatal("expected error")
		}
	}
}
func TestDailyParsing(t *testing.T) {
	c := fixture(t, `{"chart":{"result":[{"meta":{"symbol":"TEST","exchangeTimezoneName":"America/New_York"},"timestamp":[1704205800,1704292200],"indicators":{"quote":[{"open":[10,null],"high":[12,null],"low":[9,null],"close":[11,null],"volume":[100,null]}],"adjclose":[{"adjclose":[10.5,null]}]}}]}}`)
	rows, err := c.Prices(context.Background(), "TEST")
	if err != nil || len(rows) != 1 || *rows[0].Close != 11 || *rows[0].AdjustedClose != 10.5 {
		t.Fatalf("%v %v", rows, err)
	}
}
func TestCalendarKeepsZeroAndUnknownSession(t *testing.T) {
	c := fixture(t, `{"finance":{"result":[{"documents":[{"columns":[{"id":"ticker"},{"id":"startdatetime"},{"id":"startdatetimetype"},{"id":"epsactual"},{"id":"epsestimate"}],"rows":[["TEST","2025-01-03T00:00:00Z","TAS",0,null]]}]}]}}`)
	rows, err := c.Calendar(context.Background(), time.Now(), time.Now())
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	e := rows[0].Event
	if e.ReportDate.Format("2006-01-02") != "2025-01-03" || e.Session != "UNKNOWN" || e.EPSActual == nil || *e.EPSActual != 0 || e.EPSEstimate != nil {
		t.Fatalf("%+v", e)
	}
}
func Test404DoesNotRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; http.Error(w, "unavailable", 404) }))
	defer srv.Close()
	c := New()
	c.Base = srv.URL
	c.HTTP.Interval = 0
	_, err := c.Prices(context.Background(), "TEST")
	if err == nil || calls != 1 {
		t.Fatal(err, calls)
	}
}
func TestFinancialPeriodIsNotFiscalLabel(t *testing.T) {
	c := fixture(t, `{"timeseries":{"result":[{"quarterlyTotalRevenue":[{"asOfDate":"2025-01-31","currencyCode":"USD","reportedValue":{"raw":123}}]}]}}`)
	rows, err := c.Financials(context.Background(), "TEST")
	if err != nil || len(rows) != 1 || rows[0].FiscalQuarter != nil || rows[0].DilutedEPS != nil {
		t.Fatal(rows, err)
	}
}
func TestCancellation(t *testing.T) {
	c := fixture(t, `{}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.Search(ctx, "TEST")
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatal(err)
	}
}
