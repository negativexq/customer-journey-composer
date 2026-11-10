package main

import (
	"math"
	"testing"
)

func TestReportedQuantilesUseInterpolatedSamples(t *testing.T) {
	p := percentiles([]float64{50, 10, 40, 20, 30})
	if p.Count != 5 || p.P50 != 30 || math.Abs(p.P95-48) > 1e-8 || math.Abs(p.P99-49.6) > 1e-8 || p.Max != 50 {
		t.Fatal(p)
	}
	if percentiles(nil).Count != 0 {
		t.Fatal("empty samples misreported")
	}
}
