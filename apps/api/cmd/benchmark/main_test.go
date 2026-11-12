package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
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

func TestObservationReconciliationRejectsIncompleteProjection(t *testing.T) {
	for _, tc := range []struct {
		name      string
		accepted  int
		unique    int
		complete  bool
		wantError bool
	}{
		{"reconciled", 10, 10, true, false},
		{"delivery_lag", 9, 10, true, true},
		{"distinct_mismatch", 10, 9, true, true},
		{"backfill_incomplete", 10, 10, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := observationResponse{Unique: &tc.unique, Complete: tc.complete, Metrics: []cohortMetric{
				{Dimension: "execution", Metric: "entries", Value: 10},
				{Dimension: "execution", Metric: "COMPLETED", Value: 10},
				{Dimension: "edge", Key: "check:false", Metric: "traversed", Value: 10},
				{Dimension: "channel", Key: "sms", Metric: "ACCEPTED", Value: float64(tc.accepted)},
				{Dimension: "channel", Key: "sms", Metric: "scheduled", Value: 10},
				{Dimension: "node_delivery", Key: "sms", Metric: "ACCEPTED", Value: 10},
			}}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("version") != "1" {
					t.Error("missing immutable version scope")
				}
				_ = json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			_, err := verifyJourneyObservation(context.Background(), server.Client(), server.URL, "fixture", 10)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v, expected error=%v", err, tc.wantError)
			}
		})
	}
}
