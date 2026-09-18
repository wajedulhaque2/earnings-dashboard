// Package universe defines a reproducible, attribute-based production universe.
package universe

import (
	"earnings-dashboard/internal/models"
	"regexp"
	"sort"
)

const Limit = 1000

var excluded = regexp.MustCompile(`(?i)\b(preferred|preference|warrants?|units?|rights|ETF|ETN|funds?)\b`)

func Reason(c models.Company) string {
	if c.SecurityType == nil || *c.SecurityType != "EQUITY" {
		return "not_common_equity"
	}
	if c.ExchangeCode == nil {
		return "exchange_unavailable"
	}
	switch *c.ExchangeCode {
	case "NYQ", "NMS", "NGM", "NCM", "ASE":
	default:
		return "outside_supported_exchanges"
	}
	if c.Name == nil {
		return "security_name_unavailable"
	}
	if excluded.MatchString(*c.Name) {
		return "non_operating_security"
	}
	if c.Currency == nil || *c.Currency != "USD" {
		return "usd_market_cap_unavailable"
	}
	if c.MarketCap == nil || *c.MarketCap < 1e9 {
		return "below_1b_or_cap_unavailable"
	}
	return "eligible"
}

// The largest 1,000 qualifying issuers are tracked; ties use symbol order.
func Select(rows []models.Company) []models.Company {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i].MarketCap, rows[j].MarketCap
		if a == nil {
			return false
		}
		if b == nil {
			return true
		}
		if *a == *b {
			return rows[i].Symbol < rows[j].Symbol
		}
		return *a > *b
	})
	n := 0
	for i := range rows {
		rows[i].UniverseEligible = false
		rows[i].UniverseEligible = false
		rows[i].UniverseReason = Reason(rows[i])
		if rows[i].UniverseReason == "eligible" {
			if n < Limit {
				rows[i].UniverseEligible = true
				n++
			} else {
				rows[i].UniverseReason = "outside_top_1000_market_cap"
			}
		}
	}
	return rows
}
