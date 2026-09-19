package analytics

import "math"

// RevenueSurprise is a fraction. Missing or zero estimates cannot define it.
func RevenueSurprise(actual, estimate *float64) *float64 {
	if actual == nil || estimate == nil || *estimate == 0 || math.IsNaN(*actual) || math.IsNaN(*estimate) || math.IsInf(*actual, 0) || math.IsInf(*estimate, 0) {
		return nil
	}
	v := (*actual - *estimate) / math.Abs(*estimate)
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil
	}
	return &v
}
