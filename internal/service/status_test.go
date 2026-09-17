package service

import (
	"bytes"
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
	"earnings-dashboard/internal/providers/httpclient"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

type optionalSEC struct {
	calls int
	err   error
}

func (p *optionalSEC) Name() string                           { return "sec" }
func (p *optionalSEC) Eligible(context.Context, string) error { return p.err }
func (p *optionalSEC) Financials(context.Context, string) ([]models.Financial, error) {
	p.calls++
	return []models.Financial{{}}, nil
}
func (p *optionalSEC) Filings(context.Context, string) ([]models.Filing, error) {
	p.calls++
	return nil, nil
}
func (m *memoryStore) Company(_ context.Context, symbol string) (models.Company, error) {
	return m.rows[symbol], nil
}
func (m *memoryStore) UpsertFinancial(context.Context, models.Financial) error { return nil }
func TestOptionalEligibility(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status string
		fails  bool
	}{{providers.ErrUnsupported, "unsupported", false}, {providers.ErrNotConfigured, "not_attempted", true}, {&httpclient.StatusError{Code: 403}, "error", true}, {nil, "success", false}} {
		m := &memoryStore{rows: map[string]models.Company{"TEST": {ID: 1, Symbol: "TEST"}}}
		p := &optionalSEC{err: tc.err}
		var log bytes.Buffer
		s := Sync{Store: m, Fundamentals: []providers.Fundamentals{p}, Filings: p, Logger: slog.New(slog.NewJSONHandler(&log, nil))}
		err := s.SyncFinancials(context.Background(), " test ")
		if (err != nil) != tc.fails || len(m.statuses) != 2 || m.statuses[0] != tc.status || m.statuses[1] != tc.status {
			t.Fatalf("%s: %v %v", tc.status, err, m.statuses)
		}
		if tc.err != nil && p.calls != 0 {
			t.Fatal("ineligible SEC operation called downstream provider")
		}
		if tc.status == "unsupported" && (strings.Contains(log.String(), `"level":"WARN"`) || strings.Contains(log.String(), `"level":"ERROR"`)) {
			t.Fatal("unsupported logged as failure")
		}
	}
}
func TestErrorCategories(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{{&httpclient.StatusError{Code: 403}, "access_denied"}, {&httpclient.StatusError{Code: 429}, "rate_limited"}, {context.DeadlineExceeded, "timeout"}, {errors.New("secret credential"), "refresh_failed"}} {
		if got := errorCategory(tc.err); got != tc.want {
			t.Fatal(got, tc.want)
		}
	}
}
