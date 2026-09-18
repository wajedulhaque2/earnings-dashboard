package service

import (
	"context"
	"earnings-dashboard/internal/analytics"
	"earnings-dashboard/internal/models"
	"time"
)

type ReactionStore interface {
	Company(context.Context, string) (models.Company, error)
	Events(context.Context, int64) ([]models.Event, error)
	Prices(context.Context, int64) ([]models.Price, error)
	Snapshots(context.Context, int64) ([]models.Snapshot, error)
	UpsertReaction(context.Context, models.Reaction) error
}

func Recalculate(ctx context.Context, store ReactionStore, symbol string, now time.Time) error {
	c, err := store.Company(ctx, symbol)
	if err != nil {
		return err
	}
	events, err := store.Events(ctx, c.ID)
	if err != nil {
		return err
	}
	prices, err := store.Prices(ctx, c.ID)
	if err != nil {
		return err
	}
	for _, e := range events {
		r := models.Reaction{EventID: e.ID, Methodology: analytics.Methodology}
		if c.Timezone != nil && *c.Timezone == "America/New_York" {
			r = analytics.Calculate(e, prices, nil, now)
		}
		if err = store.UpsertReaction(ctx, r); err != nil {
			return err
		}
	}
	return nil
}
