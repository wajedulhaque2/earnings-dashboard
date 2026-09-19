package yahoo

import (
	"context"
	"earnings-dashboard/internal/models"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

type fiscalHistory struct {
	QuoteSummary struct {
		Result []struct {
			Earnings struct {
				EarningsChart struct {
					Quarterly []struct {
						FiscalQuarter string
						PeriodEndDate struct{ Raw int64 }
						ReportedDate  struct{ Raw int64 }
					}
				}
			}
		}
	}
}

var fiscalQuarterPattern = regexp.MustCompile(`^([1-4])Q([0-9]{4})$`)

// Yahoo's historical chart includes explicit fiscal labels and period ends.
// Calendar-quarter labels and all current/forward estimate fields are ignored.
func (c *Client) labelFinancialPeriods(ctx context.Context, symbol string, rows []models.Financial) {
	var history fiscalHistory
	if c.request(ctx, "GET", "/v10/finance/quoteSummary/"+url.PathEscape(symbol)+"?modules=earnings", nil, &history) != nil {
		return
	}
	applyFiscalHistory(rows, history, time.Now())
}
func applyFiscalHistory(rows []models.Financial, history fiscalHistory, now time.Time) {
	type label struct {
		year, quarter int
		ambiguous     bool
	}
	labels := map[string]label{}
	for _, result := range history.QuoteSummary.Result {
		for _, v := range result.Earnings.EarningsChart.Quarterly {
			parts := fiscalQuarterPattern.FindStringSubmatch(v.FiscalQuarter)
			if len(parts) != 3 || v.PeriodEndDate.Raw <= 0 || v.ReportedDate.Raw <= 0 {
				continue
			}
			end, report := time.Unix(v.PeriodEndDate.Raw, 0).UTC(), time.Unix(v.ReportedDate.Raw, 0).UTC()
			if !report.Before(now) || !report.After(end) || report.Sub(end) > 120*24*time.Hour {
				continue
			}
			q, _ := strconv.Atoi(parts[1])
			y, _ := strconv.Atoi(parts[2])
			if y < 1900 {
				continue
			}
			key := end.Format("2006-01-02")
			p := label{year: y, quarter: q}
			if old, ok := labels[key]; ok {
				p.ambiguous = old.ambiguous || old.year != y || old.quarter != q
			}
			labels[key] = p
		}
	}
	for i := range rows {
		if p, ok := labels[rows[i].PeriodEnd.Format("2006-01-02")]; ok && !p.ambiguous {
			rows[i].FiscalYear = models.Ptr(p.year)
			rows[i].FiscalQuarter = models.Ptr(p.quarter)
		}
	}
}
