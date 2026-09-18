package service

import (
	"context"
	"earnings-dashboard/internal/models"
	"earnings-dashboard/internal/universe"
	"errors"
)

type UniverseProvider interface {
	Universe(context.Context) ([]models.Company, error)
}

func (s *Sync) SyncUniverse(ctx context.Context) error {
	return s.operation(ctx, "yahoo", "universe", "", func() error {
		p, ok := s.Earnings.(UniverseProvider)
		if !ok {
			return errors.New("universe provider unavailable")
		}
		store, ok := s.Store.(interface {
			ApplyUniverse(context.Context, []models.Company) error
		})
		if !ok {
			return errors.New("universe storage unavailable")
		}
		rows, err := p.Universe(ctx)
		if err != nil {
			return err
		}
		return store.ApplyUniverse(ctx, universe.Select(rows))
	})
}
