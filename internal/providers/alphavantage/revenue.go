// Package alphavantage uses only free structured fundamental-data endpoints.
package alphavantage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
)

var ErrBudget = errors.New("free revenue-provider request budget exhausted")

type Cache interface {
	RevenueCache(context.Context, string, string) ([]byte, error)
	SaveRevenueCache(context.Context, string, string, []byte) error
	ReserveRevenueRequest(context.Context) (bool, error)
}
type Client struct {
	Key, Base string
	HTTP      *http.Client
	Cache     Cache
	Interval  time.Duration
	mu        sync.Mutex
	last      time.Time
}

func New(key string, cache Cache) *Client {
	return &Client{Key: key, Base: "https://www.alphavantage.co/query", HTTP: &http.Client{Timeout: 20 * time.Second}, Cache: cache, Interval: 14 * time.Second}
}
func (c *Client) Name() string { return "alphavantage" }

func (c *Client) request(ctx context.Context, symbol, operation string) ([]byte, error) {
	if c.Key == "" {
		return nil, providers.ErrNotConfigured
	}
	if operation != "EARNINGS_ESTIMATES" && operation != "EARNINGS" {
		return nil, providers.ErrUnsupported
	}
	if c.Cache == nil {
		return nil, providers.ErrNotConfigured
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if b, err := c.Cache.RevenueCache(ctx, symbol, operation); err != nil {
		return nil, err
	} else if len(b) > 0 {
		var cached struct{ Unavailable bool }
		_ = json.Unmarshal(b, &cached)
		if cached.Unavailable {
			return nil, providers.ErrUnavailable
		}
		return b, nil
	}
	b, err := c.download(ctx, symbol, operation)
	if err != nil {
		return nil, err
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(b, &envelope) != nil {
		return nil, providers.ErrMalformed
	}
	var got string
	_ = json.Unmarshal(envelope["symbol"], &got)
	if got != symbol {
		return nil, providers.ErrUnavailable
	}
	key := "estimates"
	if operation == "EARNINGS" {
		key = "quarterlyEarnings"
	}
	if len(envelope[key]) == 0 {
		// A recognized company with no history gets a short negative cache.
		// Never persist provider error text, which may contain account details.
		safe, _ := json.Marshal(map[string]any{"symbol": symbol, "unavailable": true})
		if err = c.Cache.SaveRevenueCache(ctx, symbol, operation, safe); err != nil {
			return nil, err
		}
		return nil, providers.ErrUnavailable
	}
	// Persist only the public fields needed, never an error message or API key.
	safe, _ := json.Marshal(map[string]any{"symbol": got, key: envelope[key]})
	if err = c.Cache.SaveRevenueCache(ctx, symbol, operation, safe); err != nil {
		return nil, err
	}
	return safe, nil
}

// At most two transient attempts, each consuming its own durable reservation.
// A retry doubles the spacing; provider quota/access messages are not retried.
func (c *Client) download(ctx context.Context, symbol, operation string) ([]byte, error) {
	for attempt := 0; attempt < 2; attempt++ {
		timer := time.NewTimer(time.Until(c.last.Add(c.Interval * time.Duration(1<<attempt))))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
		allowed, err := c.Cache.ReserveRevenueRequest(ctx)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrBudget
		}
		c.last = time.Now()
		u := c.Base + "?" + url.Values{"function": {operation}, "symbol": {symbol}, "apikey": {c.Key}}.Encode()
		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return nil, providers.ErrMalformed
		}
		res, err := c.HTTP.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		b, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		res.Body.Close()
		if res.StatusCode == 429 || res.StatusCode >= 500 || readErr != nil {
			continue
		}
		if res.StatusCode != 200 {
			return nil, providers.ErrUnavailable
		}
		return b, nil
	}
	return nil, providers.ErrUnavailable
}

type estimateResponse struct {
	Symbol    string
	Estimates []struct {
		Date, Horizon string
		Revenue       string `json:"revenue_estimate_average"`
		EPS           string `json:"eps_estimate_average"`
	}
}
type earningsResponse struct {
	Symbol            string
	QuarterlyEarnings []struct{ FiscalDateEnding, ReportedDate, EstimatedEPS, ReportedEPS string }
}

// HistoricalRevenue excludes all forecasts and annual rows. The same fiscal
// end must have an actual reported EPS and a report date in EARNINGS; its EPS
// consensus must agree with EARNINGS_ESTIMATES. No observation timestamp is invented.
func (c *Client) HistoricalRevenue(ctx context.Context, symbol string) ([]models.RevenueConsensus, error) {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return nil, err
	}
	a, err := c.request(ctx, symbol, "EARNINGS_ESTIMATES")
	if err != nil {
		return nil, err
	}
	b, err := c.request(ctx, symbol, "EARNINGS")
	if err != nil {
		return nil, err
	}
	return parseHistorical(a, b, time.Now())
}
func parseHistorical(a, b []byte, now time.Time) ([]models.RevenueConsensus, error) {
	var estimates estimateResponse
	var history earningsResponse
	if json.Unmarshal(a, &estimates) != nil || json.Unmarshal(b, &history) != nil || estimates.Symbol == "" || estimates.Symbol != history.Symbol {
		return nil, providers.ErrMalformed
	}
	out := []models.RevenueConsensus{}
	seen := map[string]bool{}
	for _, v := range estimates.Estimates {
		if v.Horizon != "fiscal quarter" {
			continue
		}
		end, err := time.Parse("2006-01-02", v.Date)
		if err != nil || !end.Before(now) {
			continue
		}
		revenue, ok := finite(v.Revenue)
		if !ok {
			continue
		}
		eps, ok := finite(v.EPS)
		if !ok {
			continue
		}
		matches := []models.RevenueConsensus{}
		for _, h := range history.QuarterlyEarnings {
			if h.FiscalDateEnding != v.Date {
				continue
			}
			report, err := time.Parse("2006-01-02", h.ReportedDate)
			if err != nil || !report.Before(now.UTC().Truncate(24*time.Hour)) || !report.After(end) || report.Sub(end) > 120*24*time.Hour {
				continue
			}
			prior, ok := finite(h.EstimatedEPS)
			// EARNINGS rounds many recent consensuses to cents while the
			// estimates endpoint retains four decimals. Compare at common precision.
			if !ok || math.Round(prior*100) != math.Round(eps*100) {
				continue
			}
			if _, ok = finite(h.ReportedEPS); !ok {
				continue
			}
			matches = append(matches, models.RevenueConsensus{Symbol: estimates.Symbol, PeriodEnd: end, ReportDate: report, Value: revenue, Source: "alphavantage"})
		}
		if len(matches) == 1 {
			if seen[v.Date] {
				return nil, providers.ErrMalformed
			}
			seen[v.Date] = true
			out = append(out, matches[0])
		}
	}
	return out, nil
}
func finite(s string) (float64, bool) {
	n, e := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return n, e == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}
