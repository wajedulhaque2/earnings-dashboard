package analytics

import "earnings-dashboard/internal/models"

// Behaviour uses independent non-null samples for every return horizon.
// Large EPS surprises also belong to the broad beat/miss group.
func Behaviour(rows []models.History, gap bool) []Group {
	names := []string{"EPS Beat", "EPS Miss", "Large EPS Beat (> +10%)", "Large EPS Miss (< -10%)"}
	if gap {
		names = []string{"Gap > +5%", "Gap +2% to +5%", "Gap -2% to +2%", "Gap -5% to -2%", "Gap < -5%"}
	}
	out := []Group{}
	for i, name := range names {
		g := Group{Name: name}
		var opening, event, week, month []*float64
		for _, r := range rows {
			match := false
			if gap {
				match = GapBucket(r.Reaction.Returns[0]) == i
			} else if Valid(r.Event.EPSSurprisePct) {
				v := *r.Event.EPSSurprisePct
				match = (i == 0 && v > 0) || (i == 1 && v < 0) || (i == 2 && v > 10) || (i == 3 && v < -10)
			}
			if !match {
				continue
			}
			g.Events++
			opening = append(opening, r.Reaction.Returns[0])
			event = append(event, r.Reaction.Returns[1])
			week = append(week, r.Reaction.Returns[4])
			month = append(month, r.Reaction.Returns[6])
		}
		g.Opening = Summarize(opening)
		g.Event = Summarize(event)
		g.Week = Summarize(week)
		g.Month = Summarize(month)
		if g.Opening.N+g.Event.N+g.Week.N+g.Month.N > 0 {
			out = append(out, g)
		}
	}
	return out
}
