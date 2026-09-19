package service

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fiscalStoreFixture struct {
	memoryStore
	links int
}

func (s *fiscalStoreFixture) LinkFiscalLabels(context.Context, int64) error     { s.links++; return nil }
func (s *fiscalStoreFixture) UpsertEvent(context.Context, models.Event) error   { return nil }
func (s *fiscalStoreFixture) UpsertFiling(context.Context, models.Filing) error { return nil }

type earningsFixture struct{}

func (earningsFixture) Name() string { return "yahoo" }
func (earningsFixture) Earnings(context.Context, string) ([]models.Event, error) {
	return []models.Event{{}}, nil
}
func (earningsFixture) Calendar(context.Context, time.Time, time.Time) ([]models.CalendarItem, error) {
	return nil, nil
}
func TestFiscalBackfillAfterIndependentSyncs(t *testing.T) {
	store := &fiscalStoreFixture{memoryStore: memoryStore{rows: map[string]models.Company{"TEST": {ID: 1, Symbol: "TEST"}}}}
	sec := &optionalSEC{}
	syncer := Sync{Store: store, Fundamentals: []providers.Fundamentals{sec}, Filings: sec, Earnings: earningsFixture{}, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for _, fn := range []func(context.Context, string) error{syncer.SyncFinancials, syncer.SyncFilings, syncer.SyncEarnings} {
		before := store.links
		if err := fn(context.Background(), "TEST"); err != nil {
			t.Fatal(err)
		}
		if store.links != before+1 {
			t.Fatal("backfill not called after ingestion")
		}
	}
}
