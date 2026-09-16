package earnings

import (
	"earnings-dashboard/internal/models"
	"testing"
	"time"
)

func TestEventSessions(t *testing.T) {
	for _, tc := range []struct{ date, session, want string }{{"2026-09-15", "AMC", "2026-09-16"}, {"2026-09-16", "BMO", "2026-09-16"}, {"2026-09-04", "AMC", "2026-09-08"}, {"2026-04-02", "AMC", "2026-04-06"}, {"2025-01-08", "AMC", "2025-01-10"}, {"2026-09-06", "BMO", "2026-09-08"}} {
		d, _ := time.Parse("2006-01-02", tc.date)
		got, ok := EventSession(models.Event{ReportDate: d, Session: tc.session})
		if !ok || got.Format("2006-01-02") != tc.want {
			t.Fatal(tc, got, ok)
		}
	}
	if _, ok := EventSession(models.Event{Session: "UNKNOWN"}); ok {
		t.Fatal("unknown timing")
	}
}
func TestHolidays(t *testing.T) {
	for _, v := range []string{"2026-01-01", "2026-01-19", "2026-02-16", "2026-04-03", "2026-05-25", "2026-06-19", "2026-07-03", "2026-09-07", "2026-11-26", "2026-12-25", "2012-10-29"} {
		d, _ := time.Parse("2006-01-02", v)
		if IsSession(d) {
			t.Fatal(v)
		}
	}
	d, _ := time.Parse("2006-01-02", "2027-12-31")
	if !IsSession(d) {
		t.Fatal("NYSE Saturday New Year must not close Friday")
	}
}
