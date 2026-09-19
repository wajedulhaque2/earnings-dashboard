package yahoo

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Opt-in diagnostic: JSON endpoints only; never runs in ordinary tests.
func TestProbeStructuredRevenue(t *testing.T) {
	if os.Getenv("YAHOO_REVENUE_PROBE") == "" {
		t.Skip("live diagnostic")
	}
	c := New()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	out := map[string]any{}
	for _, field := range []string{"epsestimate", "revenueestimate", "revenueactual", "revenuedifference", "revenuesurprise", "revenuesurprisepct", "revenueEstimate", "revenueActual", "revenueSurprise", "revenueSurprisePercent", "revenueconsensus", "consensusrevenue"} {
		body := map[string]any{"entityIdType": "sp_earnings", "size": 5, "offset": 0, "includeFields": []string{"ticker", "startdatetime", field}, "query": op("and", op("eq", "ticker", "NVDA"))}
		var r any
		err := c.request(ctx, "POST", "/v1/finance/visualization?lang=en-US&region=US", body, &r)
		if err != nil {
			out["field/"+field] = err.Error()
		} else {
			out["field/"+field] = r
		}
	}
	var ts any
	err := c.request(ctx, "GET", "/ws/fundamentals-timeseries/v1/finance/timeseries/NVDA?type=quarterlyTotalRevenue,quarterlyRevenueEstimate,quarterlyRevenueActual,quarterlyRevenueSurprise,quarterlyRevenueSurprisePercent&period1=1451606400&period2=1798761600", nil, &ts)
	if err != nil {
		out["timeseries"] = err.Error()
	} else {
		out["timeseries"] = ts
	}
	for _, symbol := range []string{"NVDA", "ADBE", "COST", "ASML"} {
		for _, module := range []string{"earningsHistory", "earnings", "earningsTrend", "calendarEvents", "incomeStatementHistoryQuarterly"} {
			var r any
			err := c.request(ctx, "GET", "/v10/finance/quoteSummary/"+symbol+"?modules="+module, nil, &r)
			if err != nil {
				out[symbol+"/"+module] = "unavailable"
			} else {
				out[symbol+"/"+module] = r
			}
		}
		for _, fields := range [][]string{nil, {"ticker", "eventname", "startdatetime", "revenueestimate", "revenueactual", "revenuedifference", "revenuesurprise", "revenuesurprisepct", "revenueEstimate", "revenueActual", "revenueSurprisePercent", "consensusrevenue"}} {
			body := map[string]any{"entityIdType": "sp_earnings", "sortType": "DESC", "sortField": "startdatetime", "size": 100, "offset": 0, "query": op("and", op("eq", "ticker", symbol), op("lte", "startdatetime", time.Now().Format("2006-01-02")))}
			key := symbol + "/visualization-default"
			if fields != nil {
				body["includeFields"] = fields
				key = symbol + "/visualization-revenue"
			}
			var r any
			err := c.request(ctx, "POST", "/v1/finance/visualization?lang=en-US&region=US", body, &r)
			if err != nil {
				out[key] = "unavailable"
			} else {
				out[key] = r
			}
		}
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile("../../../.tmp/yahoo-revenue-probe.json", b, 0600); err != nil {
		t.Fatal(err)
	}
}
