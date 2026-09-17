package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"earnings-dashboard/internal/analytics"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/repository"
	assets "earnings-dashboard/web"
	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type PageStore interface {
	Company(context.Context, string) (models.Company, error)
	Search(context.Context, string) ([]models.Company, error)
	Calendar(context.Context, repository.CalendarFilter) ([]models.CalendarItem, error)
	Events(context.Context, int64) ([]models.Event, error)
	Financials(context.Context, int64) ([]models.Financial, error)
	Reactions(context.Context, int64) (map[int64]models.Reaction, error)
	Watchlist(context.Context) ([]models.Company, error)
	SetWatchlist(context.Context, string, bool) error
	Queue(context.Context, string) error
	SyncStates(context.Context) ([]models.SyncState, error)
}
type Searcher interface {
	Search(context.Context, string) ([]models.Company, error)
}
type App struct {
	Store    PageStore
	Search   Searcher
	Location *time.Location
	pages    *template.Template
	Timeout  time.Duration
}
type day struct {
	Date            time.Time
	BMO, AMC, Other []models.CalendarItem
}
type page struct {
	WatchRows                                                                         []watchRow
	Summary                                                                           [8]analytics.Stats
	Conditions, Gaps                                                                  []analytics.Group
	Title, Message, Query, Sector, Session, MinCap, Week, Previous, Next, Chart, CSRF string
	Days                                                                              []day
	WatchOnly                                                                         bool
	Company                                                                           models.Company
	Companies                                                                         []models.Company
	Financials                                                                        []models.Financial
	History                                                                           []models.History
	NextEvent                                                                         *models.Event
	States                                                                            []models.SyncState
}
type watchRow struct {
	Company models.Company
	Next    *models.Event
}

func text(v *string) string {
	if v == nil || *v == "" {
		return "N/A"
	}
	return *v
}
func number(v *float64) string {
	if v == nil {
		return "N/A"
	}
	return strconv.FormatFloat(*v, 'f', 2, 64)
}
func percent(v *float64) string {
	if v == nil {
		return "N/A"
	}
	return fmt.Sprintf("%+.2f%%", *v*100)
}
func percentPoints(v *float64) string {
	if v == nil {
		return "N/A"
	}
	return fmt.Sprintf("%+.2f%%", *v)
}
func heat(v *float64) string {
	if v == nil || *v < 0.00005 && *v > -0.00005 {
		return "neutral"
	}
	if *v > 0 {
		return "positive"
	}
	return "negative"
}
func compact(v *float64) string {
	if v == nil {
		return "N/A"
	}
	n := *v
	for _, x := range []struct {
		n float64
		s string
	}{{1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "K"}} {
		if n >= x.n {
			return fmt.Sprintf("%.2f%s", n/x.n, x.s)
		}
	}
	return number(v)
}
func quarter(e models.Event) string {
	if e.FiscalYear != nil && e.FiscalQuarter != nil {
		return fmt.Sprintf("%d Q%d", *e.FiscalYear, *e.FiscalQuarter)
	}
	if e.PeriodEnd != nil {
		return e.PeriodEnd.Format("2006-01-02")
	}
	return "N/A"
}
func (a *App) register(r chi.Router) {
	if a.Location == nil {
		a.Location, _ = time.LoadLocation("America/New_York")
	}
	if a.Timeout == 0 {
		a.Timeout = 10 * time.Second
	}
	a.pages = template.Must(template.New("pages").Funcs(template.FuncMap{"text": text, "num": number, "pct": percent, "points": percentPoints, "heat": heat, "compact": compact, "quarter": quarter, "date": func(t time.Time) string { return t.Format("Jan 02, 2006") }, "timestamp": func(t *time.Time) string {
		if t == nil {
			return "N/A"
		}
		return t.In(a.Location).Format("Jan 02, 2006 15:04:05 MST")
	}}).ParseFS(assets.Files, "templates/*.html"))
	r.Get("/calendar", a.calendar)
	r.Get("/stocks/{symbol}", a.stock)
	r.Get("/search", a.search)
	r.Get("/watchlist", a.watchlist)
	r.Get("/status", a.status)
	r.Get("/methodology", func(w http.ResponseWriter, r *http.Request) {
		a.render(w, 200, "methodology", page{Title: "Earnings methodology"})
	})
	r.Post("/watchlist", a.watch)
	r.Post("/sync", a.queue)
}
func (a *App) render(w http.ResponseWriter, status int, name string, p page) {
	var b bytes.Buffer
	if err := a.pages.ExecuteTemplate(&b, name, p); err != nil {
		http.Error(w, "Unable to render page", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}
func (a *App) fail(w http.ResponseWriter, status int, msg string) {
	a.render(w, status, "error", page{Title: "Data unavailable", Message: msg})
}
func (a *App) calendar(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), a.Timeout)
	defer cancel()
	now := time.Now().In(a.Location)
	date := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if input := r.URL.Query().Get("week"); input != "" {
		var err error
		date, err = time.Parse("2006-01-02", input)
		if err != nil {
			a.fail(w, 400, "Invalid week date.")
			return
		}
	}
	monday := date.AddDate(0, 0, -(int(date.Weekday())+6)%7)
	q := r.URL.Query()
	p := page{Title: "Weekly earnings calendar", Week: monday.Format("2006-01-02"), Previous: monday.AddDate(0, 0, -7).Format("2006-01-02"), Next: monday.AddDate(0, 0, 7).Format("2006-01-02"), Query: q.Get("q"), Sector: q.Get("sector"), Session: q.Get("session"), MinCap: q.Get("cap"), WatchOnly: q.Get("watchlist") == "1"}
	caps := map[string]float64{"": 0, "500M": 5e8, "1B": 1e9, "5B": 5e9, "10B": 1e10, "50B": 5e10}
	cap, ok := caps[p.MinCap]
	if !ok || (p.Session != "" && p.Session != "BMO" && p.Session != "AMC" && p.Session != "UNKNOWN") || len(p.Query) > 100 || len(p.Sector) > 100 {
		a.fail(w, 400, "Invalid calendar filter.")
		return
	}
	rows, err := a.Store.Calendar(ctx, repository.CalendarFilter{From: monday, To: monday.AddDate(0, 0, 7), Session: p.Session, Sector: p.Sector, Query: p.Query, MinCap: cap, Watchlist: p.WatchOnly})
	if err != nil {
		a.fail(w, 503, "Database unavailable. Please try again shortly.")
		return
	}
	for i := 0; i < 7; i++ {
		d := day{Date: monday.AddDate(0, 0, i)}
		for _, row := range rows {
			if row.Event.ReportDate.Format("2006-01-02") != d.Date.Format("2006-01-02") {
				continue
			}
			switch row.Event.Session {
			case "BMO":
				d.BMO = append(d.BMO, row)
			case "AMC":
				d.AMC = append(d.AMC, row)
			default:
				d.Other = append(d.Other, row)
			}
		}
		if i < 5 || len(d.BMO)+len(d.AMC)+len(d.Other) > 0 {
			p.Days = append(p.Days, d)
		}
	}
	if len(rows) == 0 {
		p.Message = "No earnings data loaded for this week and these filters. Check refresh status or run sync-calendar."
	}
	a.render(w, 200, "calendar", p)
}
func (a *App) stock(w http.ResponseWriter, r *http.Request) {
	symbol, err := models.Symbol(chi.URLParam(r, "symbol"))
	if err != nil {
		a.fail(w, 400, "Invalid ticker symbol.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), a.Timeout)
	defer cancel()
	c, err := a.Store.Company(ctx, symbol)
	if errors.Is(err, pgx.ErrNoRows) {
		a.fail(w, 404, "Company not loaded. Search free sources or request a refresh from Search.")
		return
	}
	if err != nil {
		a.fail(w, 503, "Database unavailable.")
		return
	}
	events, err := a.Store.Events(ctx, c.ID)
	if err != nil {
		a.fail(w, 503, "Earnings history unavailable.")
		return
	}
	financials, err := a.Store.Financials(ctx, c.ID)
	if err != nil {
		a.fail(w, 503, "Financial history unavailable.")
		return
	}
	reactions, err := a.Store.Reactions(ctx, c.ID)
	if err != nil {
		a.fail(w, 503, "Reaction history unavailable.")
		return
	}
	p := page{Title: c.Symbol + " earnings analysis", Company: c, Financials: financials}
	today := time.Now().In(a.Location).Format("2006-01-02")
	for _, e := range events {
		if e.ReportDate.Format("2006-01-02") >= today {
			if p.NextEvent == nil || e.ReportDate.Before(p.NextEvent.ReportDate) {
				v := e
				p.NextEvent = &v
			}
		}
		p.History = append(p.History, models.History{Event: e, Reaction: reactions[e.ID]})
	}
	chart := map[string]any{"financials": financials, "history": p.History}
	p.Summary = analytics.Summary(p.History)
	p.Conditions = analytics.Groups(p.History, false)
	p.Gaps = analytics.Groups(p.History, true)
	encoded, _ := json.Marshal(chart)
	p.Chart = string(encoded)
	a.render(w, 200, "stock", p)
}
func (a *App) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) > 100 {
		a.fail(w, 400, "Search is limited to 100 characters.")
		return
	}
	p := page{Title: "Company search", Query: q}
	if q == "" {
		a.render(w, 200, "search", p)
		return
	}
	timeout := a.Timeout
	if r.URL.Query().Get("remote") == "1" {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	var err error
	p.Companies, err = a.Store.Search(ctx, q)
	if err != nil {
		a.fail(w, 503, "Database unavailable.")
		return
	}
	if r.URL.Query().Get("remote") == "1" && a.Search != nil {
		found, e := a.Search.Search(ctx, q)
		if e != nil {
			p.Message = "Free provider unavailable. Stored results are shown; try again later."
		} else if len(found) > 0 {
			p.Companies = found
		}
	}
	if len(p.Companies) == 0 && p.Message == "" {
		p.Message = "No matching companies stored. Search free sources, or queue a symbol refresh below."
	}
	a.render(w, 200, "search", p)
}
func (a *App) watchlist(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), a.Timeout)
	defer cancel()
	rows, err := a.Store.Watchlist(ctx)
	if err != nil {
		a.fail(w, 503, "Database unavailable.")
		return
	}
	p := page{Title: "Watchlist", Companies: rows}
	today := time.Now().In(a.Location).Format("2006-01-02")
	for _, c := range rows {
		row := watchRow{Company: c}
		events, e := a.Store.Events(ctx, c.ID)
		if e != nil {
			a.fail(w, 503, "Watchlist earnings unavailable.")
			return
		}
		for _, event := range events {
			if event.ReportDate.Format("2006-01-02") >= today && (row.Next == nil || event.ReportDate.Before(row.Next.ReportDate)) {
				v := event
				row.Next = &v
			}
		}
		p.WatchRows = append(p.WatchRows, row)
	}
	a.render(w, 200, "watchlist", p)
}
func (a *App) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), a.Timeout)
	defer cancel()
	rows, err := a.Store.SyncStates(ctx)
	if err != nil {
		a.fail(w, 503, "Database unavailable.")
		return
	}
	a.render(w, 200, "status", page{Title: "Data refresh status", States: rows})
}
func (a *App) watch(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		a.fail(w, 403, "Cross-site request rejected.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		a.fail(w, 400, "Invalid form.")
		return
	}
	symbol, err := models.Symbol(r.Form.Get("symbol"))
	if err != nil {
		a.fail(w, 400, "Invalid symbol.")
		return
	}
	action := r.Form.Get("action")
	if action != "add" && action != "remove" {
		a.fail(w, 400, "Invalid action.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), a.Timeout)
	defer cancel()
	if _, err = a.Store.Company(ctx, symbol); err != nil {
		a.fail(w, 404, "Load this company through Search first.")
		return
	}
	if err = a.Store.SetWatchlist(ctx, symbol, action == "add"); err != nil {
		a.fail(w, 503, "Watchlist could not be updated.")
		return
	}
	http.Redirect(w, r, "/watchlist", 303)
}
func (a *App) queue(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		a.fail(w, 403, "Cross-site request rejected.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := r.ParseForm(); err != nil {
		a.fail(w, 400, "Invalid form.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), a.Timeout)
	defer cancel()
	if _, err := models.Symbol(r.Form.Get("symbol")); err != nil {
		a.fail(w, 400, "Invalid ticker symbol.")
		return
	}
	if err := a.Store.Queue(ctx, r.Form.Get("symbol")); err != nil {
		a.fail(w, 503, "Could not queue refresh.")
		return
	}
	a.render(w, 202, "error", page{Title: "Refresh queued", Message: "The worker will ingest this symbol. Check refresh status and search again after the worker runs."})
}
