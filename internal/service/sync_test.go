package service

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
	"errors"
	"io"
	"log/slog"
	"testing"
)

type memoryStore struct {
	Store
	rows     map[string]models.Company
	statuses []string
}

func (m *memoryStore) UpsertCompany(_ context.Context, c models.Company) (models.Company, error) {
	old := m.rows[c.Symbol]
	if c.Name == nil {
		c.Name = old.Name
	}
	if c.MarketCap == nil {
		c.MarketCap = old.MarketCap
	}
	c.ID = 1
	m.rows[c.Symbol] = c
	return c, nil
}
func (m *memoryStore) RecordSync(_ context.Context, p, r, status string) error {
	m.statuses = append(m.statuses, status)
	return nil
}

type fakeReference struct {
	company models.Company
	err     error
}

func (f *fakeReference) Name() string                                             { return "yahoo" }
func (f *fakeReference) Search(context.Context, string) ([]models.Company, error) { return nil, nil }
func (f *fakeReference) Company(context.Context, string) (models.Company, error) {
	return f.company, f.err
}
func TestRepeatedSyncAndProviderFailure(t *testing.T) {
	m := &memoryStore{rows: map[string]models.Company{}}
	p := &fakeReference{company: models.Company{Symbol: "TEST", Name: models.Text("Fixture"), MarketCap: models.Ptr(100.0)}}
	s := Sync{Store: m, References: []providers.Reference{p}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if err := s.SyncCompany(ctx, " test "); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.rows) != 1 {
		t.Fatal("not idempotent")
	}
	p.company = models.Company{Symbol: "TEST"}
	if err := s.SyncCompany(ctx, "TEST"); err != nil {
		t.Fatal(err)
	}
	if *m.rows["TEST"].MarketCap != 100 {
		t.Fatal("lost observation")
	}
	p.err = errors.New("unavailable")
	if err := s.SyncCompany(ctx, "TEST"); err == nil {
		t.Fatal("failure hidden")
	}
	if len(m.rows) != 1 || m.statuses[len(m.statuses)-1] != "error" {
		t.Fatal("bad failure state")
	}
}
