package scheduler

import (
	"context"
	"earnings-dashboard/internal/models"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeStore struct {
	Store
	attempts []string
	finished []string
}

func (s *fakeStore) CalendarDue(context.Context) (bool, error)         { return false, nil }
func (s *fakeStore) Requests(context.Context) ([]string, error)        { return []string{"TEST"}, nil }
func (s *fakeStore) DueSymbols(context.Context) ([]string, error)      { return []string{"TEST"}, nil }
func (s *fakeStore) IntradaySymbols(context.Context) ([]string, error) { return nil, nil }
func (s *fakeStore) Company(context.Context, string) (models.Company, error) {
	return models.Company{ID: 1, Symbol: "TEST"}, nil
}
func (s *fakeStore) Events(context.Context, int64) ([]models.Event, error)       { return nil, nil }
func (s *fakeStore) Prices(context.Context, int64) ([]models.Price, error)       { return nil, nil }
func (s *fakeStore) Snapshots(context.Context, int64) ([]models.Snapshot, error) { return nil, nil }
func (s *fakeStore) RecordSync(_ context.Context, provider, resource, status string, categories ...string) error {
	s.attempts = append(s.attempts, status)
	return nil
}
func (s *fakeStore) FinishRequest(_ context.Context, symbol string, success bool) error {
	s.finished = append(s.finished, symbol)
	return nil
}

type fakeSync struct{ calls int }

func (s *fakeSync) SyncAll(context.Context, string) error                            { s.calls++; return nil }
func (s *fakeSync) SyncRecentIntraday(context.Context, string) error                 { return nil }
func (s *fakeSync) SyncUpcomingCalendar(context.Context, time.Time, time.Time) error { return nil }
func TestCycleDeduplicatesRequests(t *testing.T) {
	store := &fakeStore{}
	sync := &fakeSync{}
	s := Scheduler{Store: store, Sync: sync, Location: time.UTC, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := s.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sync.calls != 1 || len(store.finished) != 1 || len(store.attempts) != 1 {
		t.Fatal("duplicate scheduled work")
	}
}
func TestRunCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := Scheduler{}
	if err := s.Run(ctx); err != nil {
		t.Fatal(err)
	}
}
