package sec

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestQuarterFacts(t *testing.T) {
	var r factsResponse
	err := json.Unmarshal([]byte(`{"facts":{"us-gaap":{"Revenues":{"units":{"USD":[{"start":"2024-01-01","end":"2024-03-31","filed":"2024-05-01","form":"10-Q","val":100},{"start":"2024-01-01","end":"2024-03-31","filed":"2025-05-01","form":"10-Q","val":110},{"start":"2024-01-01","end":"2024-06-30","filed":"2024-08-01","form":"10-Q","val":230}]}},"EarningsPerShareDiluted":{"units":{"USD/shares":[{"start":"2024-01-01","end":"2024-03-31","filed":"2024-05-01","form":"10-Q","val":0}]}}}}}`), &r)
	if err != nil {
		t.Fatal(err)
	}
	out := extract(r)
	if len(out) != 1 || *out[0].Revenue != 110 || out[0].DilutedEPS == nil || *out[0].DilutedEPS != 0 || out[0].FiscalYear != nil {
		t.Fatalf("%+v", out)
	}
}
func TestTickerAndFilings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "Research contact@test.invalid" {
			t.Error("missing user agent")
		}
		if r.URL.Path == "/files/company_tickers.json" {
			_, _ = w.Write([]byte(`{"0":{"cik_str":123,"ticker":"TEST","title":"Fixture"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"name":"Fixture","tickers":["TEST"],"exchanges":["Nasdaq"],"filings":{"recent":{"accessionNumber":["000-1"],"form":["10-Q"],"filingDate":["2024-05-01"],"reportDate":["2024-03-31"],"primaryDocument":["test.htm"]}}}`))
	}))
	defer srv.Close()
	c := New("Research contact@test.invalid")
	c.WWWBase = srv.URL
	c.DataBase = srv.URL
	c.HTTP.Interval = 0
	v, err := c.Company(context.Background(), "TEST")
	if err != nil || *v.CIK != "0000000123" {
		t.Fatal(v, err)
	}
	f, err := c.Filings(context.Background(), "TEST")
	if err != nil || len(f) != 1 || f[0].Form != "10-Q" {
		t.Fatal(f, err)
	}
}
func TestMissingUserAgent(t *testing.T) {
	c := New("")
	if _, err := c.Company(context.Background(), "TEST"); err == nil {
		t.Fatal("SEC must require descriptive user agent")
	}
}
