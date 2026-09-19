package analytics

import (
	"earnings-dashboard/internal/models"
	"math"
	"testing"
)

func TestRevenueSurprise(t *testing.T) {
	for _, tc := range []struct {
		actual, estimate *float64
		want             *float64
	}{
		{models.Ptr(110.0), models.Ptr(100.0), models.Ptr(.1)},
		{models.Ptr(-90.0), models.Ptr(-100.0), models.Ptr(.1)},
		{models.Ptr(90.0), models.Ptr(100.0), models.Ptr(-.1)},
		{models.Ptr(0.0), models.Ptr(100.0), models.Ptr(-1.0)},
		{models.Ptr(100.0), models.Ptr(0.0), nil}, {nil, models.Ptr(100.0), nil}, {models.Ptr(100.0), nil, nil},
		{models.Ptr(math.Inf(1)), models.Ptr(100.0), nil}, {models.Ptr(100.0), models.Ptr(math.NaN()), nil},
	} {
		got := RevenueSurprise(tc.actual, tc.estimate)
		if (got == nil) != (tc.want == nil) || (got != nil && math.Abs(*got-*tc.want) > 1e-12) {
			t.Fatal(got, tc.want)
		}
	}
}
