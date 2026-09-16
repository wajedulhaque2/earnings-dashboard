package web

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/repository"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type pageFixture struct {
	PageStore
	err error
}

func (f pageFixture) Company(context.Context, string) (models.Company, error) {
	return models.Company{ID: 1, Symbol: "TEST", Name: models.Text("<script>alert(1)</script>")}, f.err
}
func (f pageFixture) Calendar(context.Context, repository.CalendarFilter) ([]models.CalendarItem, error) {
	return nil, f.err
}
func (f pageFixture) Events(context.Context, int64) ([]models.Event, error) { return nil, f.err }
func (f pageFixture) Financials(context.Context, int64) ([]models.Financial, error) {
	return nil, f.err
}
func (f pageFixture) Reactions(context.Context, int64) (map[int64]models.Reaction, error) {
	return nil, f.err
}
func (f pageFixture) Search(context.Context, string) ([]models.Company, error) { return nil, f.err }
func (f pageFixture) Watchlist(context.Context) ([]models.Company, error)      { return nil, f.err }
func (f pageFixture) SyncStates(context.Context) ([]models.SyncState, error)   { return nil, f.err }
func TestApplicationPages(t *testing.T) {
	r := NewRouter(readinessFunc(func(context.Context) error { return nil }), time.Second, slog.Default(), &App{Store: pageFixture{}})
	for _, path := range []string{"/calendar", "/stocks/TEST", "/search?q=TEST", "/watchlist", "/status"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "<script>alert(1)") {
			t.Fatal("unescaped company name")
		}
	}
}
func TestDatabaseFailureAndInvalidFilters(t *testing.T) {
	r := NewRouter(readinessFunc(func(context.Context) error { return nil }), time.Second, slog.Default(), &App{Store: pageFixture{err: errors.New("secret password")}})
	for _, tc := range []struct {
		path   string
		status int
	}{{"/calendar", 503}, {"/calendar?week=bad", 400}, {"/calendar?cap=NaN", 400}, {"/stocks/TEST", 503}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", tc.path, nil))
		if w.Code != tc.status || strings.Contains(w.Body.String(), "secret") {
			t.Fatalf("%s: %d", tc.path, w.Code)
		}
	}
}
func TestMutationOrigin(t *testing.T) {
	r := NewRouter(readinessFunc(func(context.Context) error { return nil }), time.Second, slog.Default(), &App{Store: pageFixture{}})
	req := httptest.NewRequest("POST", "/watchlist", strings.NewReader("symbol=TEST&action=add"))
	req.Header.Set("Origin", "https://attacker.invalid")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
