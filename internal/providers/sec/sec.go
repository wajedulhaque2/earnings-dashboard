// Package sec reads official SEC JSON, without scraping filing HTML.
package sec

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
	"earnings-dashboard/internal/providers/httpclient"
)

type ticker struct {
	CIK           int64 `json:"cik_str"`
	Ticker, Title string
}
type Client struct {
	HTTP              *httpclient.Client
	DataBase, WWWBase string
	mu                sync.Mutex
	tickers           map[string]ticker
	loaded            time.Time
}

func New(agent string) *Client {
	return &Client{HTTP: httpclient.New(agent, 250*time.Millisecond), DataBase: "https://data.sec.gov", WWWBase: "https://www.sec.gov"}
}
func (c *Client) Name() string { return "sec" }
func (c *Client) mapping(ctx context.Context) (map[string]ticker, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.loaded) < 24*time.Hour && c.tickers != nil {
		return c.tickers, nil
	}
	if !strings.Contains(c.HTTP.UserAgent, "@") || strings.Contains(c.HTTP.UserAgent, "example.com") {
		return nil, providers.ErrUnavailable
	}
	var raw map[string]ticker
	if err := c.HTTP.JSON(ctx, "GET", c.WWWBase+"/files/company_tickers.json", nil, &raw); err != nil {
		return nil, err
	}
	mapped := map[string]ticker{}
	for _, v := range raw {
		if v.Ticker != "" && v.CIK > 0 {
			mapped[strings.ToUpper(v.Ticker)] = v
		}
	}
	if len(mapped) == 0 {
		return nil, providers.ErrMalformed
	}
	c.tickers = mapped
	c.loaded = time.Now()
	return mapped, nil
}
func (c *Client) resolve(ctx context.Context, symbol string) (ticker, error) {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return ticker{}, err
	}
	m, err := c.mapping(ctx)
	if err != nil {
		return ticker{}, err
	}
	v, ok := m[symbol]
	if !ok {
		v, ok = m[strings.ReplaceAll(symbol, ".", "-")]
	}
	if !ok {
		return ticker{}, providers.ErrNotFound
	}
	return v, nil
}
func (c *Client) Search(ctx context.Context, q string) ([]models.Company, error) {
	m, err := c.mapping(ctx)
	if err != nil {
		return nil, err
	}
	q = strings.ToUpper(strings.TrimSpace(q))
	if q == "" || len(q) > 100 {
		return nil, providers.ErrNotFound
	}
	out := []models.Company{}
	for _, v := range m {
		if strings.Contains(v.Ticker, q) || strings.Contains(strings.ToUpper(v.Title), q) {
			out = append(out, models.Company{Symbol: v.Ticker, Name: models.Text(v.Title), CIK: models.Text(fmt.Sprintf("%010d", v.CIK))})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	if len(out) > 20 {
		out = out[:20]
	}
	return out, nil
}

type submission struct {
	Name               string
	Tickers, Exchanges []string
	Filings            struct {
		Recent struct{ AccessionNumber, Form, FilingDate, ReportDate, PrimaryDocument []string }
	}
}

func (c *Client) submission(ctx context.Context, symbol string) (ticker, submission, error) {
	v, err := c.resolve(ctx, symbol)
	if err != nil {
		return v, submission{}, err
	}
	var s submission
	err = c.HTTP.JSON(ctx, "GET", fmt.Sprintf("%s/submissions/CIK%010d.json", c.DataBase, v.CIK), nil, &s)
	return v, s, err
}
func (c *Client) Company(ctx context.Context, symbol string) (models.Company, error) {
	v, s, err := c.submission(ctx, symbol)
	if err != nil {
		return models.Company{}, err
	}
	company := models.Company{Symbol: symbol, Name: models.Text(s.Name), CIK: models.Text(fmt.Sprintf("%010d", v.CIK))}
	for i, t := range s.Tickers {
		if t == symbol && i < len(s.Exchanges) {
			company.Exchange = models.Text(s.Exchanges[i])
		}
	}
	return company, nil
}
func (c *Client) Filings(ctx context.Context, symbol string) ([]models.Filing, error) {
	_, s, err := c.submission(ctx, symbol)
	if err != nil {
		return nil, err
	}
	r := s.Filings.Recent
	out := []models.Filing{}
	for i, a := range r.AccessionNumber {
		if i >= len(r.Form) || i >= len(r.FilingDate) || i >= len(r.ReportDate) || i >= len(r.PrimaryDocument) {
			return nil, providers.ErrMalformed
		}
		filed, e := time.Parse("2006-01-02", r.FilingDate[i])
		if e != nil {
			continue
		}
		report, _ := time.Parse("2006-01-02", r.ReportDate[i])
		out = append(out, models.Filing{Accession: a, Form: r.Form[i], Filed: filed, ReportDate: report, Document: r.PrimaryDocument[i]})
	}
	return out, nil
}

type fact struct {
	Start, End, Filed, Form, Frame string
	Val                            *float64
}
type factsResponse struct {
	Facts map[string]map[string]struct{ Units map[string][]fact }
}

func (c *Client) Financials(ctx context.Context, symbol string) ([]models.Financial, error) {
	v, err := c.resolve(ctx, symbol)
	if err != nil {
		return nil, err
	}
	var facts factsResponse
	if err = c.HTTP.JSON(ctx, "GET", fmt.Sprintf("%s/api/xbrl/companyfacts/CIK%010d.json", c.DataBase, v.CIK), nil, &facts); err != nil {
		return nil, err
	}
	return extract(facts), nil
}
func extract(r factsResponse) []models.Financial {
	out := map[string]models.Financial{}
	// Prefer current revenue taxonomy. Only directly reported quarter-length
	// facts in USD are accepted; annual/YTD totals are never treated as quarters.
	for _, group := range []struct {
		tags []string
		unit string
		eps  bool
	}{
		{[]string{"RevenueFromContractWithCustomerExcludingAssessedTax", "RevenueFromContractWithCustomerIncludingAssessedTax", "Revenues", "SalesRevenueNet"}, "USD", false},
		{[]string{"EarningsPerShareDiluted"}, "USD/shares", true},
	} {
		chosen := map[string]fact{}
		for _, tag := range group.tags {
			latest := map[string]fact{}
			for _, f := range r.Facts["us-gaap"][tag].Units[group.unit] {
				start, e1 := time.Parse("2006-01-02", f.Start)
				end, e2 := time.Parse("2006-01-02", f.End)
				days := end.Sub(start).Hours() / 24
				if e1 != nil || e2 != nil || days < 75 || days > 105 || f.Val == nil || (f.Form != "10-Q" && f.Form != "10-K" && f.Form != "10-Q/A" && f.Form != "10-K/A") {
					continue
				}
				old, ok := latest[f.End]
				if !ok || f.Filed > old.Filed {
					latest[f.End] = f
				}
			}
			for end, f := range latest {
				if _, ok := chosen[end]; !ok {
					chosen[end] = f
				}
			}
		}
		for end, f := range chosen {
			v := out[end]
			v.PeriodEnd, _ = time.Parse("2006-01-02", end)
			v.Source = "sec"
			v.Currency = "USD"
			if group.eps {
				v.DilutedEPS = f.Val
			} else {
				v.Revenue = f.Val
			}
			out[end] = v
		}
	}
	result := []models.Financial{}
	for _, v := range out {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PeriodEnd.Before(result[j].PeriodEnd) })
	return result
}
