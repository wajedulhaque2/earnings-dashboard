// Package scheduler runs bounded, sequential refresh cycles. PostgreSQL stores
// due times; restarting the process does not reset provider cooldowns.
package scheduler

import (
	"context"
	"earnings-dashboard/internal/earnings"
	"earnings-dashboard/internal/service"
	"errors"
	"log/slog"
	"time"
)

type Store interface {
	service.ReactionStore
	Requests(context.Context) ([]string, error)
	FinishRequest(context.Context, string, bool) error
	DueSymbols(context.Context) ([]string, error)
	IntradaySymbols(context.Context) ([]string, error)
	CalendarDue(context.Context) (bool, error)
	RecordSync(context.Context, string, string, string, ...string) error
}
type Syncer interface {
	SyncAll(context.Context, string) error
	SyncRecentIntraday(context.Context, string) error
	SyncUpcomingCalendar(context.Context, time.Time, time.Time) error
}
type Scheduler struct {
	Store    Store
	Sync     Syncer
	Logger   *slog.Logger
	Location *time.Location
}

func (s *Scheduler) Cycle(ctx context.Context) error {
	var failures []error
	due, err := s.Store.CalendarDue(ctx)
	if err != nil {
		return err
	}
	if due {
		now := time.Now().In(s.Location)
		if err = s.Sync.SyncUpcomingCalendar(ctx, now.AddDate(0, 0, -7), now.AddDate(0, 0, 14)); err != nil {
			failures = append(failures, err)
		}
	}
	requests, err := s.Store.Requests(ctx)
	if err != nil {
		return err
	}
	dueSymbols, err := s.Store.DueSymbols(ctx)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, symbol := range append(requests, dueSymbols...) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if seen[symbol] {
			continue
		}
		seen[symbol] = true
		err = s.Sync.SyncAll(ctx, symbol)
		calcErr := service.Recalculate(ctx, s.Store, symbol, time.Now())
		err = errors.Join(err, calcErr)
		status := "success"
		if err != nil {
			status = "error"
			failures = append(failures, err)
		}
		if e := s.Store.RecordSync(ctx, "yahoo", "all:"+symbol, status); e != nil {
			failures = append(failures, e)
		}
		if e := s.Store.FinishRequest(ctx, symbol, err == nil); e != nil {
			failures = append(failures, e)
		}
	}
	// Fetch after the premarket window so the 09:29 minute bar has completed.
	now := time.Now().In(s.Location)
	if earnings.IsSession(now) && now.Hour()*60+now.Minute() >= 570 && now.Hour() <= 20 {
		symbols, e := s.Store.IntradaySymbols(ctx)
		if e != nil {
			failures = append(failures, e)
		} else {
			for _, symbol := range symbols {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				if e = s.Sync.SyncRecentIntraday(ctx, symbol); e != nil {
					failures = append(failures, e)
				}
				if e = service.Recalculate(ctx, s.Store, symbol, time.Now()); e != nil {
					failures = append(failures, e)
				}
			}
		}
	}
	return errors.Join(failures...)
}
func (s *Scheduler) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		cycle, cancel := context.WithTimeout(ctx, 30*time.Minute)
		err := s.Cycle(cycle)
		cancel()
		if err != nil {
			s.Logger.Warn("refresh cycle incomplete", "status", "error", "error", "refresh_failed")
		}
		timer := time.NewTimer(time.Minute)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
