package analytics

import (
	"earnings-dashboard/internal/models"
	"math"
	"testing"
)

func TestDailyOpeningGapSessions(t *testing.T) {
	prices := []models.Price{{Date: date("2026-09-08"), Close: models.Ptr(100.0)}, {Date: date("2026-09-09"), Open: models.Ptr(104.0), Close: models.Ptr(102.0)}}
	for _, e := range []models.Event{{ReportDate: date("2026-09-08"), Session: "AMC"}, {ReportDate: date("2026-09-09"), Session: "BMO"}} {
		r := Calculate(e, prices, nil, date("2026-09-11"))
		if r.Returns[0] == nil || math.Abs(*r.Returns[0]-.04) > 1e-10 || r.EventDate.Format("2006-01-02") != "2026-09-09" {
			t.Fatal(r)
		}
	}
	prices[1].Open = nil
	r := Calculate(models.Event{ReportDate: date("2026-09-09"), Session: "BMO"}, prices, nil, date("2026-09-11"))
	if r.Returns[0] != nil || r.Returns[1] == nil {
		t.Fatal("missing open must not remove event close return")
	}
}
func TestEPSBehaviourWithoutRevenue(t *testing.T) {
	rows := []models.History{
		{Event: models.Event{EPSSurprisePct: models.Ptr(11.0)}, Reaction: models.Reaction{Returns: [8]*float64{models.Ptr(.03), models.Ptr(.04)}}},
		{Event: models.Event{EPSSurprisePct: models.Ptr(10.0)}, Reaction: models.Reaction{Returns: [8]*float64{nil, models.Ptr(-.02)}}},
		{Event: models.Event{EPSSurprisePct: models.Ptr(-11.0)}, Reaction: models.Reaction{Returns: [8]*float64{models.Ptr(-.03), models.Ptr(-.04)}}},
	}
	groups := Behaviour(rows, false)
	if len(groups) != 4 || groups[0].Events != 2 || groups[0].Opening.N != 1 || groups[2].Events != 1 || *groups[0].Event.WinRate != .5 {
		t.Fatal(groups)
	}
	if len(Behaviour(nil, false)) != 0 || len(Behaviour(nil, true)) != 0 {
		t.Fatal("empty grids must be hidden")
	}
}
