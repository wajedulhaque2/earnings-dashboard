package analytics

import (
	"earnings-dashboard/internal/earnings"
	"earnings-dashboard/internal/models"
	"math"
	"testing"
	"time"
)

func date(s string) time.Time { v, _ := time.Parse("2006-01-02", s); return v }
func TestReturnsAndMissingSessions(t *testing.T) {
	event := models.Event{ID: 1, ReportDate: date("2026-09-04"), Session: "AMC"}
	prices := []models.Price{{Date: date("2026-09-04"), Close: models.Ptr(100.0)}, {Date: date("2026-09-08"), Close: models.Ptr(110.0)}, {Date: date("2026-09-09"), Close: models.Ptr(121.0)}}
	r := Calculate(event, prices, nil, date("2026-09-15"))
	if math.Abs(*r.Returns[1]-.1) > 1e-9 || math.Abs(*r.Returns[2]-.1) > 1e-9 || r.Returns[3] != nil || r.Returns[0] != nil {
		t.Fatalf("%+v", r)
	}
	if Return(models.Ptr(1.0), models.Ptr(0.0)) != nil {
		t.Fatal("zero denominator")
	}
	if Return(nil, models.Ptr(1.0)) != nil {
		t.Fatal("nil price")
	}
}
func TestAllHorizonsAndPremarket(t *testing.T) {
	d := date("2026-03-09")
	prev, _ := earnings.Shift(d, -1)
	prices := []models.Price{{Date: prev, Close: models.Ptr(100.0)}, {Date: d, Close: models.Ptr(110.0)}}
	for i, h := range Horizons {
		target, _ := earnings.Shift(d, h)
		prices = append(prices, models.Price{Date: target, Close: models.Ptr(110.0 + float64(i+1))})
	}
	loc, _ := time.LoadLocation("America/New_York")
	snap := []models.Snapshot{{Time: time.Date(2026, 3, 9, 9, 29, 0, 0, loc), Session: "PRE", Price: 105, Source: "yahoo"}, {Time: time.Date(2026, 3, 9, 9, 30, 0, 0, loc), Session: "REGULAR", Price: 999}}
	r := Calculate(models.Event{ReportDate: d, Session: "BMO"}, prices, snap, date("2027-01-01"))
	if math.Abs(*r.Returns[0]-.05) > 1e-9 {
		t.Fatal(r)
	}
	for i := range Horizons {
		if math.Abs(*r.Returns[i+2]-(float64(i+1)/110)) > 1e-9 {
			t.Fatal(i, r)
		}
	}
}
func TestSummaryAndNulls(t *testing.T) {
	s := Summarize([]*float64{nil, models.Ptr(-.1), models.Ptr(0.0), models.Ptr(.2), models.Ptr(math.NaN())})
	if s.N != 3 || *s.Median != 0 || math.Abs(*s.WinRate-1.0/3) > 1e-9 || s.StdDev == nil {
		t.Fatal(s)
	}
	if Summarize(nil).Average != nil {
		t.Fatal("empty average")
	}
	if Summarize([]*float64{models.Ptr(0.0)}).StdDev != nil {
		t.Fatal("sample deviation needs two observations")
	}
}
func TestConditionsAndGapBoundaries(t *testing.T) {
	for i, v := range [][2]float64{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
		if Condition(models.Event{EPSSurprisePct: &v[0], RevenueSurprisePct: &v[1]}) != i {
			t.Fatal(i)
		}
	}
	if Condition(models.Event{}) != -1 {
		t.Fatal("null segmentation")
	}
	for _, tc := range []struct {
		v    float64
		want int
	}{{.051, 0}, {.05, 1}, {.02, 1}, {0, 2}, {-.02, 3}, {-.05, 3}, {-.051, 4}} {
		if GapBucket(&tc.v) != tc.want {
			t.Fatal(tc)
		}
	}
}
