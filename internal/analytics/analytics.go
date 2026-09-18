// Package analytics owns every return formula and statistical definition.
package analytics

import (
	"earnings-dashboard/internal/earnings"
	"earnings-dashboard/internal/models"
	"math"
	"sort"
	"time"
)

const Methodology = "us-sessions-v2-daily-opening-gap"

var Horizons = []int{1, 2, 5, 10, 21, 63}

func Valid(v *float64) bool { return v != nil && !math.IsNaN(*v) && !math.IsInf(*v, 0) }
func Return(price, base *float64) *float64 {
	if !Valid(price) || !Valid(base) || *base <= 0 || *price <= 0 {
		return nil
	}
	v := *price / *base - 1
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
func Calculate(e models.Event, prices []models.Price, snapshots []models.Snapshot, now time.Time) models.Reaction {
	r := models.Reaction{EventID: e.ID, Methodology: Methodology}
	event, ok := earnings.EventSession(e)
	if !ok {
		return r
	}
	r.EventDate = &event
	previous, ok := earnings.Shift(event, -1)
	if !ok {
		return r
	}
	byDate := map[string]models.Price{}
	for _, p := range prices {
		byDate[p.Date.Format("2006-01-02")] = p
	}
	prev := byDate[previous.Format("2006-01-02")]
	day := byDate[event.Format("2006-01-02")]
	r.PreviousClose = prev.Close
	loc, _ := time.LoadLocation("America/New_York")
	localNow := now.In(loc)
	// Today’s daily bar is never final before the following date, including early closes.
	if event.Format("2006-01-02") < localNow.Format("2006-01-02") {
		r.EventOpen = day.Open
		r.EventClose = day.Close
		r.Returns[0] = Return(day.Open, prev.Close)
		r.Returns[1] = Return(day.Close, prev.Close)
	}
	var selected *models.Snapshot
	for i := range snapshots {
		p := &snapshots[i]
		d := p.Time.In(loc)
		minute := d.Hour()*60 + d.Minute()
		if p.Session != "PRE" || d.Format("2006-01-02") != event.Format("2006-01-02") || minute < 565 || minute >= 570 || p.Time.After(now) || p.Price <= 0 {
			continue
		}
		if selected == nil || p.Time.After(selected.Time) || (p.Time.Equal(selected.Time) && p.Source == "yahoo") {
			selected = p
		}
	}
	// Stored intraday observations are unadjusted. A later split makes them
	// incomparable with Yahoo's split-adjusted historical closes; leave N/A.
	laterSplit := false
	for _, p := range prices {
		if p.Date.After(event) && p.SplitRatio != nil {
			laterSplit = true
		}
	}
	if selected != nil && !laterSplit {
		r.PremarketPrice = models.Ptr(selected.Price)
		r.PremarketReturn = Return(r.PremarketPrice, prev.Close)
	}
	for i, h := range Horizons {
		date, ok := earnings.Shift(event, h)
		if !ok || date.Format("2006-01-02") >= localNow.Format("2006-01-02") {
			continue
		}
		p := byDate[date.Format("2006-01-02")]
		r.Returns[i+2] = Return(p.Close, r.EventClose)
	}
	return r
}

type Stats struct {
	N                                             int
	Average, Median, WinRate, Best, Worst, StdDev *float64
}

func Summarize(values []*float64) Stats {
	v := []float64{}
	sum, wins := 0.0, 0.0
	for _, p := range values {
		if Valid(p) {
			v = append(v, *p)
			sum += *p
			if *p > 0 {
				wins++
			}
		}
	}
	s := Stats{N: len(v)}
	if len(v) == 0 {
		return s
	}
	sort.Float64s(v)
	mean := sum / float64(len(v))
	median := v[len(v)/2]
	if len(v)%2 == 0 {
		median = (v[len(v)/2-1] + median) / 2
	}
	s.Average = &mean
	s.Median = &median
	s.WinRate = models.Ptr(wins / float64(len(v)))
	s.Best = models.Ptr(v[len(v)-1])
	s.Worst = models.Ptr(v[0])
	if len(v) > 1 {
		ss := 0.0
		for _, n := range v {
			ss += (n - mean) * (n - mean)
		}
		s.StdDev = models.Ptr(math.Sqrt(ss / float64(len(v)-1)))
	}
	return s
}
func Summary(rows []models.History) [8]Stats {
	var out [8]Stats
	for i := range out {
		v := []*float64{}
		for _, row := range rows {
			v = append(v, row.Reaction.Returns[i])
		}
		out[i] = Summarize(v)
	}
	return out
}
func Condition(e models.Event) int {
	if !Valid(e.EPSSurprisePct) || !Valid(e.RevenueSurprisePct) || *e.EPSSurprisePct == 0 || *e.RevenueSurprisePct == 0 {
		return -1
	}
	if *e.EPSSurprisePct > 0 {
		if *e.RevenueSurprisePct > 0 {
			return 0
		}
		return 1
	}
	if *e.RevenueSurprisePct > 0 {
		return 2
	}
	return 3
}
func GapBucket(v *float64) int {
	if !Valid(v) {
		return -1
	}
	switch {
	case *v > 0.05:
		return 0
	case *v >= 0.02:
		return 1
	case *v > -0.02:
		return 2
	case *v >= -0.05:
		return 3
	default:
		return 4
	}
}

type Group struct {
	Name                                      string
	Events                                    int
	Opening, Event, Week, Month, Continuation Stats
}

func Groups(rows []models.History, gap bool) []Group {
	names := []string{"EPS beat + revenue beat", "EPS beat + revenue miss", "EPS miss + revenue beat", "EPS miss + revenue miss"}
	if gap {
		names = []string{"Gap > +5%", "Gap +2% to +5%", "Gap -2% to +2%", "Gap -5% to -2%", "Gap < -5%"}
	}
	out := make([]Group, len(names))
	for i, name := range names {
		g := Group{Name: name}
		var event, week, month, continuation []*float64
		for _, row := range rows {
			bucket := Condition(row.Event)
			if gap {
				bucket = GapBucket(row.Reaction.Returns[0])
			}
			if bucket != i {
				continue
			}
			g.Events++
			event = append(event, row.Reaction.Returns[1])
			week = append(week, row.Reaction.Returns[4])
			month = append(month, row.Reaction.Returns[6])
			if gap && Valid(row.Reaction.Returns[0]) && Valid(row.Reaction.Returns[1]) && *row.Reaction.Returns[0] != 0 {
				v := (1+*row.Reaction.Returns[1])/(1+*row.Reaction.Returns[0]) - 1
				if *row.Reaction.Returns[0] < 0 {
					v = -v
				}
				continuation = append(continuation, &v)
			}
		}
		g.Event = Summarize(event)
		g.Week = Summarize(week)
		g.Month = Summarize(month)
		g.Continuation = Summarize(continuation)
		out[i] = g
	}
	return out
}
