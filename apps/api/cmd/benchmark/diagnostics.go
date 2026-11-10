package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The candidate keeps the ordered due-job index as the outer access path.
// It matches the production guard and does not update or claim any rows.
const orderedClaimCandidate = `SELECT j.id FROM scheduled_jobs j
 WHERE j.state='SCHEDULED' AND j.due_at<=$1
 AND (SELECT e.state IN ('RUNNING','WAITING') AND f.status IN ('Active','Sunset')
      FROM journey_executions e JOIN journeys f ON f.id=e.journey_id WHERE e.id=j.execution_id)
 ORDER BY j.due_at,j.id LIMIT 32 FOR UPDATE OF j SKIP LOCKED`

func queryStats(ctx context.Context, pool *pgxpool.Pool) (json.RawMessage, error) {
	var raw []byte
	err := pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(s)),'[]'::jsonb) FROM (
 SELECT queryid::text, left(query,2000) AS query, calls, total_exec_time, mean_exec_time, rows,
 shared_blks_hit,shared_blks_read,temp_blks_written
 FROM pg_stat_statements WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database())
 AND query NOT LIKE '%pg_stat_statements%' ORDER BY total_exec_time DESC LIMIT 25)s`).Scan(&raw)
	return raw, err
}

type readProbe struct {
	mu       sync.Mutex
	phase    atomic.Value
	samples  map[string][]float64
	failures map[string]int
	stop     context.CancelFunc
	done     chan struct{}
}

// Five reads/sec, below the workspace UI limiter. Reads run while ingestion,
// steps, outbox delivery and projection compete for the same database CPU.
func probeReads(ctx context.Context, client *http.Client, base, journey string) *readProbe {
	ctx, cancel := context.WithCancel(ctx)
	p := &readProbe{samples: map[string][]float64{}, failures: map[string]int{}, stop: cancel, done: make(chan struct{})}
	p.phase.Store("ingestion")
	paths := []struct{ name, path string }{
		{"overview", "/overview"}, {"analytics", "/analytics?days=30"}, {"journey_list", "/journeys?limit=50"},
		{"customer_track", "/journeys/" + journey + "/executions?customer_id=bench_000001"}, {"entries_24h", "/overview/entries-24h"},
	}
	go func() {
		defer close(p.done)
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				route := paths[i%len(paths)]
				i++
				phase := p.phase.Load().(string)
				start := time.Now()
				_, code, err := request(ctx, client, base, route.path, "GET", nil, "", "")
				if ctx.Err() != nil {
					return
				}
				p.mu.Lock()
				if err != nil || code != 200 {
					p.failures[fmt.Sprintf("%s: HTTP %d", route.name, code)]++
				} else {
					key := phase + "/" + route.name
					p.samples[key] = append(p.samples[key], float64(time.Since(start).Microseconds())/1000)
				}
				p.mu.Unlock()
			}
		}
	}()
	return p
}
func (p *readProbe) finish() (map[string]distribution, map[string]int) {
	p.stop()
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	result := map[string]distribution{}
	for key, samples := range p.samples {
		result[key] = percentiles(samples)
	}
	return result, p.failures
}
