package sec

import (
	"context"
	"earnings-dashboard/internal/providers"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEligibilityAndUSFilers(t *testing.T) {
	// Official CIK identities are reference fixtures, not financial observations.
	known := map[string]int64{"NVDA": 1045810, "META": 1326801, "MU": 723125, "LEN": 920760, "AAPL": 320193, "MSFT": 789019, "AMZN": 1018724, "GOOGL": 1652044, "TSLA": 1318605, "AMD": 2488}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/company_tickers.json" {
			calls++
			t.Errorf("unexpected unsupported downstream call: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{`)
		i := 0
		for symbol, cik := range known {
			if i > 0 {
				fmt.Fprint(w, ",")
			}
			fmt.Fprintf(w, `"%d":{"ticker":%q,"cik_str":%d}`, i, symbol, cik)
			i++
		}
		fmt.Fprint(w, `,"10":{"ticker":"BRK-B","cik_str":1067983},"11":{"ticker":"INVALID","cik_str":0}}`)
	}))
	defer srv.Close()
	c := New("Fixture contact@test.invalid")
	c.WWWBase = srv.URL
	c.DataBase = srv.URL
	c.HTTP.Interval = 0
	for symbol, want := range known {
		got, err := c.ResolveCIK(context.Background(), " "+symbol+" ")
		if err != nil || got != fmt.Sprintf("%010d", want) {
			t.Errorf("%s: %s %v", symbol, got, err)
		}
	}
	if cik, err := c.ResolveCIK(context.Background(), " brk.b "); err != nil || cik != "0001067983" {
		t.Fatal(cik, err)
	}
	for _, symbol := range []string{"UNMAPPED.F", "UNMAPPEDY", "INVALID"} {
		if _, err := c.Financials(context.Background(), symbol); !errors.Is(err, providers.ErrUnsupported) {
			t.Fatal(symbol, err)
		}
		if _, err := c.Filings(context.Background(), symbol); !errors.Is(err, providers.ErrUnsupported) {
			t.Fatal(symbol, err)
		}
	}
	if calls != 0 {
		t.Fatal("unsupported symbols called SEC data APIs")
	}
}
func TestMappingFailureIsNotUnsupported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "denied", 403) }))
	defer srv.Close()
	c := New("Fixture contact@test.invalid")
	c.WWWBase = srv.URL
	c.HTTP.Interval = 0
	_, err := c.ResolveCIK(context.Background(), "NVDA")
	if err == nil || errors.Is(err, providers.ErrUnsupported) {
		t.Fatal("mapping outage must remain an error", err)
	}
}
