package universe

import (
	"earnings-dashboard/internal/models"
	"fmt"
	"testing"
)

func candidate(symbol string, cap float64) models.Company {
	return models.Company{Symbol: symbol, Name: models.Text("Operating Corporation"), SecurityType: models.Text("EQUITY"), ExchangeCode: models.Text("NMS"), Currency: models.Text("USD"), MarketCap: models.Ptr(cap)}
}
func TestEligibilityAndCapRanking(t *testing.T) {
	for _, tc := range []struct{ field, value string }{{"exchange", "PNK"}, {"type", "ETF"}, {"name", "Example Preferred Shares"}, {"name", "Example Warrant"}, {"name", "Example Units"}, {"name", "Example Fund"}} {
		c := candidate("TEST", 2e9)
		switch tc.field {
		case "exchange":
			c.ExchangeCode = &tc.value
		case "type":
			c.SecurityType = &tc.value
		case "name":
			c.Name = &tc.value
		}
		if Reason(c) == "eligible" {
			t.Fatal(tc)
		}
	}
	if Reason(candidate("SMALL", 999999999)) == "eligible" {
		t.Fatal("cap threshold")
	}
	rows := []models.Company{}
	for i := 0; i < 1002; i++ {
		rows = append(rows, candidate(fmt.Sprintf("X%04d", i), 1e9+float64(i)))
	}
	rows = Select(rows)
	n := 0
	for _, c := range rows {
		if c.UniverseEligible {
			n++
		}
	}
	if n != 1000 || rows[0].Symbol != "X1001" || rows[1001].UniverseReason != "outside_top_1000_market_cap" {
		t.Fatal(n)
	}
}
