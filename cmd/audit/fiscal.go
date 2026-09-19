package main

import (
	"context"
	"earnings-dashboard/internal/repository"
	"encoding/json"
	"os"
	"time"
)

type fiscalCounts struct {
	Events          int `json:"events"`
	Quarter         int `json:"quarter"`
	RevenueActual   int `json:"revenue_actual"`
	RevenueEstimate int `json:"revenue_estimate"`
	RevenueSurprise int `json:"revenue_surprise"`
}
type fiscalCompany struct {
	Symbol string `json:"symbol"`
	fiscalCounts
	Sources               map[string]map[string]int `json:"sources"`
	MissingQuarterReasons map[string]int            `json:"missing_quarter_reasons"`
}

func auditFiscalRevenue(ctx context.Context, store *repository.Store, symbols []string, path, asOf string, failures []string) error {
	loc, _ := time.LoadLocation("America/New_York")
	if asOf == "" {
		asOf = time.Now().In(loc).Format("2006-01-02")
	}
	cutoff, err := time.Parse("2006-01-02", asOf)
	if err != nil {
		return err
	}
	report := struct {
		AsOf      string          `json:"exclusive_report_date_cutoff"`
		Companies []fiscalCompany `json:"companies"`
		Totals    fiscalCounts    `json:"totals"`
		Failures  []string        `json:"refresh_failures"`
	}{AsOf: asOf, Failures: failures}
	for _, symbol := range symbols {
		c, err := store.Company(ctx, symbol)
		if err != nil {
			return err
		}
		if !c.UniverseEligible {
			continue
		}
		events, err := store.Events(ctx, c.ID)
		if err != nil {
			return err
		}
		row := fiscalCompany{Symbol: symbol, Sources: map[string]map[string]int{}, MissingQuarterReasons: map[string]int{}}
		add := func(field string, source *string) {
			if row.Sources[field] == nil {
				row.Sources[field] = map[string]int{}
			}
			name := "legacy_unattributed"
			if source != nil {
				name = *source
			}
			row.Sources[field][name]++
		}
		for _, e := range events {
			if !e.ReportDate.Before(cutoff) {
				continue
			}
			row.Events++
			if e.FiscalYear != nil && e.FiscalQuarter != nil {
				row.Quarter++
				add("quarter", e.FiscalLabelSource)
			} else {
				reason := "not_enriched"
				if e.FiscalMappingReason != nil {
					reason = *e.FiscalMappingReason
				}
				row.MissingQuarterReasons[reason]++
			}
			if e.RevenueActual != nil {
				row.RevenueActual++
				add("revenue_actual", e.RevenueActualSource)
			}
			if e.RevenueEstimate != nil && e.RevenueEstimateSource != nil {
				row.RevenueEstimate++
				add("revenue_estimate", e.RevenueEstimateSource)
			}
			if e.RevenueSurprisePct != nil {
				row.RevenueSurprise++
				add("revenue_surprise", e.RevenueEstimateSource)
			}
		}
		report.Companies = append(report.Companies, row)
		report.Totals.Events += row.Events
		report.Totals.Quarter += row.Quarter
		report.Totals.RevenueActual += row.RevenueActual
		report.Totals.RevenueEstimate += row.RevenueEstimate
		report.Totals.RevenueSurprise += row.RevenueSurprise
	}
	b, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0644)
}
