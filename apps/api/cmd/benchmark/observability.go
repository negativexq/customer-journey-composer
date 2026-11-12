package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type observationRoute struct{ name, path string }

func observationRoutes(journey string) []observationRoute {
	prefix := "/journeys/" + journey
	return []observationRoute{
		{"journey_live", prefix + "/insights?version=1&days=0&include_unique=false"},
		{"journey_analytics", prefix + "/insights?version=1&days=30"},
		{"journey_people", prefix + "/executions?version=1&node_id=check&port=false&limit=25"},
		{"journey_messages", prefix + "/messages?version=1&node_id=sms&channel=sms&status=ACCEPTED&limit=25"},
		{"customer_track", prefix + "/executions?customer_id=bench_000001"},
		{"overview", "/overview"}, {"analytics", "/analytics?days=30"},
		{"journey_list", "/journeys?limit=50"}, {"entries_24h", "/overview/entries-24h"},
	}
}

type projectionSample struct {
	Phase         string    `json:"phase"`
	At            time.Time `json:"at"`
	Depth         int       `json:"depth_capped_at_100001"`
	OldestSeconds float64   `json:"oldest_age_seconds"`
}
type projectionProbe struct {
	Samples   []projectionSample `json:"samples"`
	Errors    int                `json:"errors"`
	PeakDepth int                `json:"peak_depth_capped_at_100001"`
	PeakAge   float64            `json:"peak_oldest_age_seconds"`
}

// One diagnostic query/sec; bounded depth prevents an unbounded queue count.
// Sampled oldest FIFO row age is freshness pressure, not per-event latency.
func (p *projectionProbe) sample(ctx context.Context, pool *pgxpool.Pool, phase string) {
	sample := projectionSample{Phase: phase, At: time.Now().UTC()}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM (SELECT 1 FROM metric_queue LIMIT 100001)q),COALESCE((SELECT greatest(0,extract(epoch FROM clock_timestamp()-created_at))::float8 FROM metric_queue ORDER BY id LIMIT 1),0)`).Scan(&sample.Depth, &sample.OldestSeconds)
	if err != nil {
		p.Errors++
		return
	}
	p.Samples = append(p.Samples, sample)
	p.PeakDepth = max(p.PeakDepth, sample.Depth)
	p.PeakAge = max(p.PeakAge, sample.OldestSeconds)
}

type cohortMetric struct {
	Dimension string  `json:"dimension"`
	Key       string  `json:"key"`
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
}
type observationResponse struct {
	Metrics  []cohortMetric `json:"metrics"`
	Unique   *int           `json:"unique_customers"`
	Complete bool           `json:"coverage_complete"`
}

func (r observationResponse) value(dimension, key, metric string) float64 {
	for _, m := range r.Metrics {
		if m.Dimension == dimension && m.Key == key && m.Metric == metric {
			return m.Value
		}
	}
	return 0
}
func verifyJourneyObservation(ctx context.Context, client *http.Client, base, journey string, customers int) (map[string]any, error) {
	raw, code, err := request(ctx, client, base, "/journeys/"+journey+"/insights?version=1&days=30", "GET", nil, "", "")
	var r observationResponse
	if err != nil || code != 200 || json.Unmarshal(raw, &r) != nil {
		return nil, fmt.Errorf("journey reconciliation response invalid: HTTP %d", code)
	}
	checks := map[string]float64{
		"entries":               r.value("execution", "", "entries"),
		"completed":             r.value("execution", "", "COMPLETED"),
		"condition_false":       r.value("edge", "check:false", "traversed"),
		"sms_accepted":          r.value("channel", "sms", "ACCEPTED"),
		"sms_logical_scheduled": r.value("channel", "sms", "scheduled"),
		"sms_node_accepted":     r.value("node_delivery", "sms", "ACCEPTED"),
	}
	for key, value := range checks {
		if value != float64(customers) {
			return nil, fmt.Errorf("cohort %s=%v, expected %d", key, value, customers)
		}
	}
	if r.Unique == nil || *r.Unique != customers || !r.Complete || r.value("execution", "", "WAITING") != 0 || r.value("execution", "", "converted") != 0 {
		return nil, fmt.Errorf("cohort distinct/coverage/state reconciliation failed")
	}
	return map[string]any{"checks": checks, "unique_customers": *r.Unique, "coverage_complete": r.Complete, "remaining_waiting": 0, "converted": 0, "pass": true}, nil
}

// These are components of the actual insight/drilldown queries, rather than
// pretending an abbreviated aggregate is the complete multi-subquery API SQL.
func addObservationPlans(ctx context.Context, pool *pgxpool.Pool, workspace, journey, version string, explain func(string, string, ...any) error, report map[string]any) error {
	from := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, -29)
	until := from.AddDate(0, 0, 30)
	for _, q := range []struct {
		name, sql string
		args      []any
	}{
		{"cohort_summary_component", `SELECT dimension,dimension_key,metric,sum(value) FROM journey_cohort_metrics WHERE workspace_id=$1 AND journey_id=$2 AND cohort_day>=$3::date AND cohort_day<$4::date AND version_id=$5 GROUP BY 1,2,3`, []any{workspace, journey, from, until, version}},
		{"cohort_distinct_component", `SELECT count(DISTINCT customer_id) FROM journey_cohort_members WHERE workspace_id=$1 AND journey_id=$2 AND cohort_day>=$3::date AND cohort_day<$4::date AND version_id=$5`, []any{workspace, journey, from, until, version}},
		{"journey_message_page", `SELECT d.id,d.execution_id,e.customer_id,e.version_id,d.node_id,d.channel,d.status,d.decision,d.attempts,d.created_at,d.last_error,e.started_at FROM journey_executions e JOIN delivery_jobs d ON d.workspace_id=e.workspace_id AND d.execution_id=e.id WHERE e.workspace_id=$1 AND e.journey_id=$2 AND e.version_id=$3 AND d.mode='LIVE' AND d.node_id='sms' AND d.channel='sms' AND d.status='ACCEPTED' ORDER BY d.created_at DESC,d.id DESC LIMIT 25`, []any{workspace, journey, version}},
		{"journey_branch_people_page", `SELECT id,state,started_at FROM journey_executions WHERE workspace_id=$1 AND journey_id=$2 AND version_id=$3 AND EXISTS(SELECT 1 FROM step_executions s WHERE s.workspace_id=journey_executions.workspace_id AND s.execution_id=journey_executions.id AND s.node_id='check' AND s.state='SUCCEEDED' AND s.output->>'port'='false') ORDER BY started_at DESC,id DESC LIMIT 25`, []any{workspace, journey, version}},
	} {
		if err := explain(q.name, q.sql, q.args...); err != nil {
			return err
		}
	}
	var storage []byte
	err := pool.QueryRow(ctx, `SELECT jsonb_agg(jsonb_build_object('table',relname,'rows_estimate',n_live_tup,'inserts',n_tup_ins,'updates',n_tup_upd,'hot_updates',n_tup_hot_upd,'deletes',n_tup_del,'total_bytes',pg_total_relation_size(relid))) FROM pg_stat_user_tables WHERE relname IN ('metric_queue','metric_snapshot_facts','journey_cohort_metrics','journey_cohort_members','journey_executions','step_executions','delivery_jobs')`).Scan(&storage)
	if err != nil {
		return err
	}
	report["observation_storage"] = json.RawMessage(storage)
	return nil
}

// Names and paths contain synthetic journey identity only, never credentials.
func observationRoutesReport(journey string) map[string]string {
	result := map[string]string{}
	for _, r := range observationRoutes(journey) {
		result[r.name] = r.path
	}
	return result
}
