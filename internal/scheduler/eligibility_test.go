package scheduler

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/providers"
	"earnings-dashboard/internal/service"
	"io"
	"log/slog"
	"testing"
	"time"
)

type unsupportedFundamentals struct{}

func (unsupportedFundamentals) Name() string { return "sec" }
func (unsupportedFundamentals) Eligible(context.Context, string) error {
	return providers.ErrUnsupported
}
func (unsupportedFundamentals) Financials(context.Context, string) ([]models.Financial, error) {
	panic("unsupported operation called")
}

type eligibilityStore struct {
	service.Store
	statuses []string
}

func (s *eligibilityStore) Company(context.Context, string) (models.Company, error) {
	return models.Company{ID: 1, Symbol: "TEST"}, nil
}
func (s *eligibilityStore) RecordSync(_ context.Context, _, _, status string, _ ...string) error {
	s.statuses = append(s.statuses, status)
	return nil
}

type optionalRefresh struct {
	fakeSync
	sync *service.Sync
}

func (s *optionalRefresh) SyncAll(ctx context.Context, symbol string) error {
	return s.sync.SyncFinancials(ctx, symbol)
}
func TestUnsupportedOptionalProviderDoesNotFailCycle(t *testing.T) {
	state := &fakeStore{}
	optional := &eligibilityStore{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	syncer := &service.Sync{Store: optional, Fundamentals: []providers.Fundamentals{unsupportedFundamentals{}}, Logger: logger}
	scheduler := Scheduler{Store: state, Sync: &optionalRefresh{sync: syncer}, Logger: logger, Location: time.UTC}
	if err := scheduler.Cycle(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(optional.statuses) != 1 || optional.statuses[0] != "unsupported" || len(state.attempts) != 1 || state.attempts[0] != "success" {
		t.Fatal(optional.statuses, state.attempts)
	}
}
