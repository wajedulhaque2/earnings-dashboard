// Package yahoo isolates Yahoo's unofficial HTTP endpoints and response formats.
package yahoo

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"earnings-dashboard/internal/earnings"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
	"earnings-dashboard/internal/providers/httpclient"
)

type Client struct {
	HTTP                      *httpclient.Client
	Base, AuthBase, CookieURL string
	mu                        sync.Mutex
	crumb                     string
}

func New() *Client {
	h := httpclient.New("Mozilla/5.0 (compatible; EarningsDashboard/1.0)", time.Second)
	h.HTTP.Jar, _ = cookiejar.New(nil)
	return &Client{HTTP: h, Base: "https://query1.finance.yahoo.com", AuthBase: "https://query1.finance.yahoo.com", CookieURL: "https://fc.yahoo.com"}
}
func (c *Client) Name() string { return "yahoo" }
func (c *Client) request(ctx context.Context, method, path string, body, out any) error {
	c.mu.Lock()
	crumb := c.crumb
	c.mu.Unlock()
	withCrumb := func(value string) string {
		if value == "" {
			return c.Base + path
		}
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		return c.Base + path + sep + "crumb=" + url.QueryEscape(value)
	}
	err := c.HTTP.JSON(ctx, method, withCrumb(crumb), body, out)
	var status *httpclient.StatusError
	if !errors.As(err, &status) || status.Code != 401 {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	// Refresh only in response to authentication failure; cookies remain in memory.
	_, _ = c.HTTP.Do(ctx, "GET", c.CookieURL, nil)
	fresh, e := c.HTTP.Do(ctx, "GET", c.AuthBase+"/v1/test/getcrumb", nil)
	if e != nil {
		return providers.ErrUnavailable
	}
	c.crumb = strings.TrimSpace(string(fresh))
	if c.crumb == "" || len(c.crumb) > 128 || strings.ContainsAny(c.crumb, "<>\n") {
		return providers.ErrUnavailable
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return c.HTTP.JSON(ctx, method, c.Base+path+sep+"crumb="+url.QueryEscape(c.crumb), body, out)
}

type number struct {
	Raw *float64 `json:"raw"`
}

func valid(p *float64) *float64 {
	if p == nil || math.IsNaN(*p) || math.IsInf(*p, 0) {
		return nil
	}
	return p
}
func positive(p *float64) *float64 {
	if p = valid(p); p == nil || *p <= 0 {
		return nil
	}
	return p
}
func (c *Client) Search(ctx context.Context, q string) ([]models.Company, error) {
	if len(strings.TrimSpace(q)) == 0 || len(q) > 100 {
		return nil, providers.ErrNotFound
	}
	var r struct {
		Quotes []struct{ Symbol, Shortname, Longname, Exchange, QuoteType string }
	}
	if err := c.request(ctx, "GET", "/v1/finance/search?q="+url.QueryEscape(q)+"&quotesCount=20&newsCount=0", nil, &r); err != nil {
		return nil, err
	}
	out := []models.Company{}
	for _, v := range r.Quotes {
		sym, e := models.Symbol(v.Symbol)
		if e != nil {
			continue
		}
		name := v.Longname
		if name == "" {
			name = v.Shortname
		}
		out = append(out, models.Company{Symbol: sym, Name: models.Text(name), Exchange: models.Text(v.Exchange)})
	}
	return out, nil
}
func (c *Client) Company(ctx context.Context, symbol string) (models.Company, error) {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return models.Company{}, err
	}
	var r struct {
		QuoteSummary struct {
			Error  any
			Result []struct {
				Price struct {
					Symbol, LongName, ShortName, Currency, ExchangeName string
					MarketCap, RegularMarketPrice                       number
					RegularMarketTime                                   int64
				}
				AssetProfile struct{ LongBusinessSummary, Sector, Industry, Country string }
			}
		}
	}
	err = c.request(ctx, "GET", "/v10/finance/quoteSummary/"+url.PathEscape(symbol)+"?modules=price,assetProfile", nil, &r)
	if err == nil && r.QuoteSummary.Error == nil && len(r.QuoteSummary.Result) > 0 {
		v := r.QuoteSummary.Result[0]
		if v.Price.Symbol != "" && v.Price.Symbol != symbol {
			return models.Company{}, providers.ErrMalformed
		}
		name := v.Price.LongName
		if name == "" {
			name = v.Price.ShortName
		}
		company := models.Company{Symbol: symbol, Name: models.Text(name), Description: models.Text(v.AssetProfile.LongBusinessSummary), Sector: models.Text(v.AssetProfile.Sector), Industry: models.Text(v.AssetProfile.Industry), Country: models.Text(v.AssetProfile.Country), Currency: models.Text(v.Price.Currency), Exchange: models.Text(v.Price.ExchangeName), MarketCap: valid(v.Price.MarketCap.Raw), LatestPrice: positive(v.Price.RegularMarketPrice.Raw)}
		if v.Price.RegularMarketTime > 0 {
			company.QuoteTime = models.Ptr(time.Unix(v.Price.RegularMarketTime, 0))
		}
		chart, e := c.chart(ctx, symbol, "range=5d&interval=1d")
		if e == nil {
			company.Timezone = models.Text(chart.Meta.ExchangeTimezoneName)
		}
		if company.Name != nil || company.LatestPrice != nil {
			return company, nil
		}
	}
	// Chart metadata is a real, separately sourced fallback, never a synthetic profile.
	ch, e := c.chart(ctx, symbol, "range=5d&interval=1d")
	if e != nil {
		return models.Company{}, e
	}
	name := ch.Meta.LongName
	if name == "" {
		name = ch.Meta.ShortName
	}
	company := models.Company{Symbol: symbol, Name: models.Text(name), Currency: models.Text(ch.Meta.Currency), Exchange: models.Text(ch.Meta.ExchangeName), Timezone: models.Text(ch.Meta.ExchangeTimezoneName), LatestPrice: positive(ch.Meta.RegularMarketPrice)}
	if ch.Meta.RegularMarketTime > 0 {
		company.QuoteTime = models.Ptr(time.Unix(ch.Meta.RegularMarketTime, 0))
	}
	return company, nil
}

type chartResult struct {
	Events struct {
		Splits map[string]struct {
			Date                   int64
			Numerator, Denominator float64
		}
	}
	Meta struct {
		Symbol, LongName, ShortName, Currency, ExchangeName, ExchangeTimezoneName string
		RegularMarketPrice                                                        *float64
		RegularMarketTime                                                         int64
	}
	Timestamp  []int64
	Indicators struct {
		Quote []struct {
			Open, High, Low, Close []*float64
			Volume                 []*int64
		}
		Adjclose []struct{ Adjclose []*float64 }
	}
}

func (c *Client) chart(ctx context.Context, symbol, params string) (chartResult, error) {
	var r struct {
		Chart struct {
			Result []chartResult
			Error  any
		}
	}
	err := c.request(ctx, "GET", "/v8/finance/chart/"+url.PathEscape(symbol)+"?"+params, nil, &r)
	if err != nil {
		return chartResult{}, err
	}
	if r.Chart.Error != nil || len(r.Chart.Result) == 0 {
		return chartResult{}, providers.ErrNotFound
	}
	v := r.Chart.Result[0]
	if v.Meta.Symbol != symbol {
		return chartResult{}, providers.ErrMalformed
	}
	return v, nil
}
func at[T any](v []T, i int) (z T) {
	if i < len(v) {
		return v[i]
	}
	return z
}
func (c *Client) Prices(ctx context.Context, symbol string) ([]models.Price, error) {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return nil, err
	}
	ch, err := c.chart(ctx, symbol, "range=10y&interval=1d&events=splits")
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(ch.Meta.ExchangeTimezoneName)
	if err != nil {
		return nil, providers.ErrUnavailable
	}
	if len(ch.Indicators.Quote) == 0 {
		return nil, providers.ErrUnavailable
	}
	q := ch.Indicators.Quote[0]
	out := []models.Price{}
	for i, ts := range ch.Timestamp {
		d := time.Unix(ts, 0).In(loc)
		if d.Format("2006-01-02") == time.Now().In(loc).Format("2006-01-02") {
			continue
		} // never persist a still-forming daily bar
		p := models.Price{Date: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), Open: positive(at(q.Open, i)), High: positive(at(q.High, i)), Low: positive(at(q.Low, i)), Close: positive(at(q.Close, i)), Volume: at(q.Volume, i), Source: c.Name()}
		for _, split := range ch.Events.Splits {
			if time.Unix(split.Date, 0).In(loc).Format("2006-01-02") == d.Format("2006-01-02") && split.Numerator > 0 && split.Denominator > 0 {
				p.SplitRatio = models.Ptr(split.Numerator / split.Denominator)
			}
		}
		if len(ch.Indicators.Adjclose) > 0 {
			p.AdjustedClose = positive(at(ch.Indicators.Adjclose[0].Adjclose, i))
		}
		if p.Close != nil {
			out = append(out, p)
		}
	}
	return out, nil
}
func (c *Client) Intraday(ctx context.Context, symbol string) ([]models.Snapshot, error) {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return nil, err
	}
	ch, err := c.chart(ctx, symbol, "range=5d&interval=1m&includePrePost=true")
	if err != nil {
		return nil, err
	}
	if ch.Meta.ExchangeTimezoneName != "America/New_York" || len(ch.Indicators.Quote) == 0 {
		return nil, providers.ErrUnavailable
	}
	loc, _ := time.LoadLocation("America/New_York")
	out := []models.Snapshot{}
	for i, ts := range ch.Timestamp {
		p := positive(at(ch.Indicators.Quote[0].Close, i))
		if p == nil || ts > time.Now().Add(-time.Minute).Unix() {
			continue
		}
		d := time.Unix(ts, 0).In(loc)
		m := d.Hour()*60 + d.Minute()
		session := "REGULAR"
		if m < 240 || m >= 1200 {
			continue
		}
		if m < 570 {
			session = "PRE"
		} else if m >= 960 {
			session = "POST"
		}
		out = append(out, models.Snapshot{Time: time.Unix(ts, 0), Price: *p, Session: session, Source: c.Name()})
	}
	return out, nil
}
func op(operator string, values ...any) map[string]any {
	return map[string]any{"operator": operator, "operands": values}
}
func (c *Client) calendar(ctx context.Context, from, to time.Time, symbol string) ([]models.CalendarItem, error) {
	query := op("and", op("eq", "region", "us"), op("or", op("eq", "eventtype", "EAD"), op("eq", "eventtype", "ERA")), op("gte", "startdatetime", from.Format("2006-01-02")), op("lte", "startdatetime", to.Format("2006-01-02")))
	if symbol != "" {
		query["operands"] = append(query["operands"].([]any), op("eq", "ticker", symbol))
	}
	out := []models.CalendarItem{}
	for offset := 0; offset < 10000; offset += 100 {
		body := map[string]any{"entityIdType": "sp_earnings", "sortType": "ASC", "sortField": "startdatetime", "includeFields": []string{"ticker", "companyshortname", "intradaymarketcap", "eventname", "startdatetime", "startdatetimetype", "epsestimate", "epsactual", "epssurprisepct"}, "size": 100, "offset": offset, "query": query}
		var r struct {
			Finance struct {
				Error  any
				Result []struct {
					Documents []struct {
						Columns []struct{ ID, Label, Type string }
						Rows    [][]json.RawMessage
					}
				}
			}
		}
		if err := c.request(ctx, "POST", "/v1/finance/visualization?lang=en-US&region=US", body, &r); err != nil {
			return nil, err
		}
		if r.Finance.Error != nil || len(r.Finance.Result) == 0 || len(r.Finance.Result[0].Documents) == 0 {
			return nil, providers.ErrUnavailable
		}
		doc := r.Finance.Result[0].Documents[0]
		for _, row := range doc.Rows {
			fields := map[string]json.RawMessage{}
			for i, col := range doc.Columns {
				key := col.ID
				if key == "" {
					key = col.Label
				}
				if key == "Event Start Date" && col.Type == "STRING" {
					key = "startdatetimetype"
				}
				fields[key] = at(row, i)
			}
			pick := func(keys ...string) json.RawMessage {
				for _, k := range keys {
					if v, ok := fields[k]; ok {
						return v
					}
				}
				return nil
			}
			sym, e := models.Symbol(rawText(pick("ticker", "Symbol")))
			if e != nil {
				continue
			}
			rawDate := rawText(pick("startdatetime", "Event Start Date"))
			var date time.Time
			for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02"} {
				date, e = time.Parse(layout, rawDate)
				if e == nil {
					break
				}
			}
			if e != nil {
				return nil, providers.ErrMalformed
			}
			stamp := date
			// The calendar date is the provider's report date; do not turn midnight UTC into yesterday.
			date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
			session := "UNKNOWN"
			switch strings.ToUpper(rawText(pick("startdatetimetype", "Timing"))) {
			case "BMO", "BEFORE MARKET OPEN":
				session = "BMO"
			case "AMC", "AFTER MARKET CLOSE":
				session = "AMC"
			case "DURING MARKET":
				session = "DURING_MARKET"
			}
			event := models.Event{Symbol: sym, ReportDate: date, Session: session, Source: c.Name(), EPSEstimate: rawNumber(pick("epsestimate", "EPS Estimate")), EPSActual: rawNumber(pick("epsactual", "Reported EPS")), EPSSurprisePct: rawNumber(pick("epssurprisepct", "Surprise (%)"))}
			// TAS records supply an event timestamp. TNS/unknown codes and midnight
			// date placeholders do not establish a release session.
			if rawText(pick("startdatetimetype", "Timing")) == "TAS" && strings.Contains(rawDate, "T") && (stamp.Hour() != 0 || stamp.Minute() != 0 || stamp.Second() != 0) {
				loc, _ := time.LoadLocation("America/New_York")
				event.ReportTime = &stamp
				event.ReportDate = earnings.Date(stamp.In(loc))
				event.Session = earnings.SessionAt(stamp)
			}
			if parts := quarterLabel.FindStringSubmatch(rawText(pick("eventname", "Event Name"))); len(parts) == 3 {
				q, _ := strconv.Atoi(parts[1])
				y, _ := strconv.Atoi(parts[2])
				event.FiscalQuarter = &q
				event.FiscalYear = &y
			}
			out = append(out, models.CalendarItem{Company: models.Company{Symbol: sym, Name: models.Text(rawText(pick("companyshortname", "Company Name"))), MarketCap: rawNumber(pick("intradaymarketcap", "Market Cap (Intraday)"))}, Event: event})
		}
		if len(doc.Rows) < 100 {
			return out, nil
		}
	}
	return nil, errors.New("calendar exceeds pagination limit; narrow date window")
}

var quarterLabel = regexp.MustCompile(`^Q([1-4]) ([0-9]{4}) Earnings`)

func rawText(v json.RawMessage) string { var s string; _ = json.Unmarshal(v, &s); return s }
func rawNumber(v json.RawMessage) *float64 {
	var n *float64
	if json.Unmarshal(v, &n) == nil {
		return valid(n)
	}
	s := rawText(v)
	x, e := strconv.ParseFloat(s, 64)
	if e != nil {
		return nil
	}
	return valid(&x)
}
func (c *Client) Calendar(ctx context.Context, from, to time.Time) ([]models.CalendarItem, error) {
	return c.calendar(ctx, from, to, "")
}
func (c *Client) Earnings(ctx context.Context, symbol string) ([]models.Event, error) {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	rows, err := c.calendar(ctx, now.AddDate(-10, 0, 0), now.AddDate(0, 3, 0), symbol)
	if err != nil {
		return nil, err
	}
	out := []models.Event{}
	for _, r := range rows {
		if r.Event.Symbol == symbol {
			out = append(out, r.Event)
		}
	}
	return out, nil
}
func (c *Client) Financials(ctx context.Context, symbol string) ([]models.Financial, error) {
	symbol, err := models.Symbol(symbol)
	if err != nil {
		return nil, err
	}
	var r struct {
		Timeseries struct {
			Error  any
			Result []struct {
				QuarterlyTotalRevenue []struct {
					AsOfDate, CurrencyCode string
					ReportedValue          number
				}
				QuarterlyDilutedEPS []struct {
					AsOfDate, CurrencyCode string
					ReportedValue          number
				}
			}
		}
	}
	path := "/ws/fundamentals-timeseries/v1/finance/timeseries/" + url.PathEscape(symbol) + "?type=quarterlyTotalRevenue,quarterlyDilutedEPS&period1=" + strconv.FormatInt(time.Now().AddDate(-5, 0, 0).Unix(), 10) + "&period2=" + strconv.FormatInt(time.Now().Unix(), 10)
	if err := c.request(ctx, "GET", path, nil, &r); err != nil {
		return nil, err
	}
	if r.Timeseries.Error != nil {
		return nil, providers.ErrUnavailable
	}
	byDate := map[string]models.Financial{}
	for _, item := range r.Timeseries.Result {
		for _, v := range item.QuarterlyTotalRevenue {
			d, e := time.Parse("2006-01-02", v.AsOfDate)
			if e != nil {
				continue
			}
			f := byDate[v.AsOfDate]
			f.PeriodEnd = d
			f.Source = c.Name()
			f.Revenue = valid(v.ReportedValue.Raw)
			f.Currency = v.CurrencyCode
			byDate[v.AsOfDate] = f
		}
		for _, v := range item.QuarterlyDilutedEPS {
			d, e := time.Parse("2006-01-02", v.AsOfDate)
			if e != nil {
				continue
			}
			f := byDate[v.AsOfDate]
			f.PeriodEnd = d
			f.Source = c.Name()
			f.DilutedEPS = valid(v.ReportedValue.Raw)
			if f.Currency == "" {
				f.Currency = v.CurrencyCode
			}
			byDate[v.AsOfDate] = f
		}
	}
	out := []models.Financial{}
	for _, v := range byDate {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PeriodEnd.Before(out[j].PeriodEnd) })
	if len(out) == 0 {
		return nil, providers.ErrUnavailable
	}
	return out, nil
}
