package web

import (
	"context"
	"earnings-dashboard/internal/models"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type statusFixture struct{ pageFixture }

func (statusFixture) SyncStates(context.Context, ...string) ([]models.SyncState, error) {
	first := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	latest := first.Add(24 * time.Hour)
	return []models.SyncState{{Provider: "sec", Operation: "financials", Symbol: "NVDA", LatestAttemptAt: &latest, LatestAttemptStatus: "error", LastSuccessAt: &first, LatestErrorCategory: models.Text("access_denied")}, {Provider: "sec", Operation: "filings", Symbol: "UNMAPPED", LatestAttemptAt: &latest, LatestAttemptStatus: "unsupported"}, {Provider: "sec", Operation: "financials", Symbol: "TEST", LatestAttemptStatus: "not_attempted"}}, nil
}
func TestStatusColumnsAndDistinctStates(t *testing.T) {
	r := NewRouter(readinessFunc(func(context.Context) error { return nil }), time.Second, slog.Default(), &App{Store: statusFixture{}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/status", nil))
	body := w.Body.String()
	for _, s := range []string{"Provider", "Operation", "Symbol", "Latest attempt", "Latest status", "Last success", "Error category", "NVDA", "access_denied", "status-error", "status-unsupported", "status-not_attempted", "Sep 01", "Sep 02"} {
		if !strings.Contains(body, s) {
			t.Fatalf("missing %q", s)
		}
	}
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
