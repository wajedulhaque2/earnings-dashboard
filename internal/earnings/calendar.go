// Package earnings centralizes US cash-equity trading-session dates.
package earnings

import (
	"earnings-dashboard/internal/models"
	"time"
)

func Date(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// SessionAt classifies an explicitly supplied event timestamp in New York.
// Early closes are trading sessions with a 13:00 close, not holidays.
func SessionAt(t time.Time) string {
	loc, _ := time.LoadLocation("America/New_York")
	d := t.In(loc)
	if !IsSession(d) {
		return "UNKNOWN"
	}
	minutes := d.Hour()*60 + d.Minute()
	closeMinutes := 16 * 60
	if d.Month() == time.November && Date(d).Equal(nth(d.Year(), time.November, time.Thursday, 4).AddDate(0, 0, 1)) ||
		d.Month() == time.December && d.Day() == 24 || d.Month() == time.July && d.Day() == 3 {
		closeMinutes = 13 * 60
	}
	if minutes < 9*60+30 {
		return "BMO"
	}
	if minutes >= closeMinutes {
		return "AMC"
	}
	return "DURING_MARKET"
}
func nth(y int, m time.Month, w time.Weekday, n int) time.Time {
	d := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	return d.AddDate(0, 0, (int(w)-int(d.Weekday())+7)%7+7*(n-1))
}
func last(y int, m time.Month, w time.Weekday) time.Time {
	d := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC)
	return d.AddDate(0, 0, -(int(d.Weekday())-int(w)+7)%7)
}
func observed(d time.Time) time.Time {
	switch d.Weekday() {
	case time.Saturday:
		return d.AddDate(0, 0, -1)
	case time.Sunday:
		return d.AddDate(0, 0, 1)
	}
	return d
}
func easter(y int) time.Time {
	a := y % 19
	b := y / 100
	c := y % 100
	d := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - d - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return time.Date(y, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
func IsSession(t time.Time) bool {
	d := Date(t)
	y := d.Year()
	if y < 1990 || y > 2035 || d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		return false
	}
	ny := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
	if ny.Weekday() == time.Sunday {
		ny = ny.AddDate(0, 0, 1)
	} // NYSE does not observe Saturday New Year on Friday.
	holidays := []time.Time{ny, nth(y, 2, time.Monday, 3), easter(y).AddDate(0, 0, -2), last(y, 5, time.Monday), observed(time.Date(y, 7, 4, 0, 0, 0, 0, time.UTC)), nth(y, 9, time.Monday, 1), nth(y, 11, time.Thursday, 4), observed(time.Date(y, 12, 25, 0, 0, 0, 0, time.UTC))}
	if y >= 1998 {
		holidays = append(holidays, nth(y, 1, time.Monday, 3))
	}
	if y >= 2022 {
		holidays = append(holidays, observed(time.Date(y, 6, 19, 0, 0, 0, 0, time.UTC)))
	}
	for _, h := range holidays {
		if d.Equal(h) {
			return false
		}
	}
	switch d.Format("2006-01-02") {
	case "1994-04-27", "2001-09-11", "2001-09-12", "2001-09-13", "2001-09-14", "2004-06-11", "2007-01-02", "2012-10-29", "2012-10-30", "2018-12-05", "2025-01-09":
		return false
	}
	return true
}
func Shift(date time.Time, n int) (time.Time, bool) {
	d := Date(date)
	step := 1
	if n < 0 {
		step = -1
		n = -n
	}
	for tries := 0; n > 0 && tries < 1000; tries++ {
		d = d.AddDate(0, 0, step)
		if IsSession(d) {
			n--
		}
	}
	return d, n == 0 && IsSession(d)
}
func EventSession(e models.Event) (time.Time, bool) {
	d := Date(e.ReportDate)
	switch e.Session {
	case "AMC":
		return Shift(d, 1)
	case "BMO":
		if IsSession(d) {
			return d, true
		}
		return Shift(d, 1)
	default:
		return time.Time{}, false
	}
}
