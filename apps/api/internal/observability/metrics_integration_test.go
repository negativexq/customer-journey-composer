//go:build integration

package observability_test

import (
	"context"
	"github.com/ofk/customer-journey-composer/apps/api/internal/database"
	"github.com/ofk/customer-journey-composer/apps/api/internal/observability"
	"os"
	"testing"
)

func TestQueueProbesCapCountsAndPublishFreshness(t *testing.T) {
	ctx := context.Background()
	pool, err := database.Open(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO metric_queue(workspace_id,journey_id,version_id,cohort_at,deltas,created_at) SELECT gen_random_uuid(),gen_random_uuid(),gen_random_uuid(),now(),'[]',now()-interval '20 seconds' FROM generate_series(1,10005)`); err != nil {
		t.Fatal(err)
	}
	if err = observability.SampleQueues(ctx, pool); err != nil {
		t.Fatal(err)
	}
	metrics, err := observability.Registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	observed := map[string]float64{}
	for _, family := range metrics {
		for _, m := range family.Metric {
			for _, label := range m.Label {
				if label.GetName() == "queue" && label.GetValue() == "analytics" {
					observed[family.GetName()] = m.GetGauge().GetValue()
				}
			}
		}
	}
	if observed["journey_queue_depth"] != 10000 || observed["journey_queue_depth_capped"] != 1 || observed["journey_queue_oldest_age_seconds"] < 20 {
		t.Fatal(observed)
	}
}
