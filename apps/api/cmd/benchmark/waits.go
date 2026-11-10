package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type waitSample struct {
	Phase            string  `json:"phase"`
	QueryID          string  `json:"query_id"`
	Type             string  `json:"wait_type"`
	Event            string  `json:"wait_event"`
	BackendSamples   int64   `json:"backend_samples"`
	BlockedSamples   int64   `json:"blocked_backend_samples"`
	MaxTransactionMS float64 `json:"max_transaction_age_ms"`
}
type waitProbe struct {
	Samples       map[string]*waitSample
	Ticks         int `json:"ticks"`
	Errors        int `json:"errors"`
	DroppedGroups int `json:"dropped_groups"`
}

// Dedicated diagnostic connection: no application pool capacity is consumed.
// Only normalized query IDs and wait categories survive; no SQL/args/PIDs.
func (p *waitProbe) sample(ctx context.Context, pool *pgxpool.Pool, phase string) {
	p.Ticks++
	rows, err := pool.Query(ctx, `SELECT COALESCE(query_id::text,'0'),COALESCE(wait_event_type,'CPU_or_not_waiting'),COALESCE(wait_event,''),count(*),count(*) FILTER(WHERE wait_event_type='Lock' AND cardinality(pg_blocking_pids(pid))>0),COALESCE(max(extract(epoch FROM clock_timestamp()-xact_start)*1000),0)::float8 FROM pg_stat_activity WHERE datid=(SELECT oid FROM pg_database WHERE datname=current_database()) AND pid<>pg_backend_pid() AND state='active' GROUP BY query_id,wait_event_type,wait_event`)
	if err != nil {
		p.Errors++
		return
	}
	defer rows.Close()
	for rows.Next() {
		v := waitSample{Phase: phase}
		if err = rows.Scan(&v.QueryID, &v.Type, &v.Event, &v.BackendSamples, &v.BlockedSamples, &v.MaxTransactionMS); err != nil {
			p.Errors++
			return
		}
		key := phase + "/" + v.QueryID + "/" + v.Type + "/" + v.Event
		old := p.Samples[key]
		if old == nil {
			if len(p.Samples) >= 512 {
				p.DroppedGroups++
				continue
			}
			p.Samples[key] = &v
			continue
		}
		old.BackendSamples += v.BackendSamples
		old.BlockedSamples += v.BlockedSamples
		old.MaxTransactionMS = max(old.MaxTransactionMS, v.MaxTransactionMS)
	}
	if rows.Err() != nil {
		p.Errors++
	}
}
func (p *waitProbe) report() map[string]any {
	samples := make([]waitSample, 0, len(p.Samples))
	for _, s := range p.Samples {
		samples = append(samples, *s)
	}
	sort.Slice(samples, func(i, j int) bool {
		a, b := samples[i], samples[j]
		return a.Phase+"/"+a.QueryID+"/"+a.Type+"/"+a.Event < b.Phase+"/"+b.QueryID+"/"+b.Type+"/"+b.Event
	})
	return map[string]any{"interval_ms": 200, "ticks": p.Ticks, "errors": p.Errors, "dropped_groups": p.DroppedGroups, "samples": samples, "meaning": "Sampled active-backend occupancy, not exact wait duration. CPU_or_not_waiting does not establish CPU saturation. Query IDs correlate with statement statistics; inactive transactions and diagnostic connection are excluded."}
}
func startProfiles(prefix string) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(prefix), 0755); err != nil {
		return nil, err
	}
	cpu, err := os.OpenFile(prefix+".cpu.pprof", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if err = pprof.StartCPUProfile(cpu); err != nil {
		_ = cpu.Close()
		return nil, err
	}
	var once sync.Once
	var result error
	return func() error {
		once.Do(func() {
			pprof.StopCPUProfile()
			result = cpu.Close()
			alloc, err := os.OpenFile(prefix+".allocs.pprof", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				result = err
				return
			}
			err = pprof.Lookup("allocs").WriteTo(alloc, 0)
			closeErr := alloc.Close()
			if err != nil {
				result = err
			} else if closeErr != nil {
				result = closeErr
			}
		})
		return result
	}, nil
}
func startWaitProbe(ctx context.Context, dsn string, phase func() string) (*waitProbe, func(), error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, nil, err
	}
	cfg.MaxConns = 1
	cfg.ConnConfig.RuntimeParams["application_name"] = "benchmark_diagnostics"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	p := &waitProbe{Samples: map[string]*waitSample{}}
	go func() {
		defer close(done)
		defer pool.Close()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sampleCtx, stop := context.WithTimeout(ctx, 100*time.Millisecond)
				p.sample(sampleCtx, pool, phase())
				stop()
			}
		}
	}()
	var once sync.Once
	return p, func() { once.Do(func() { cancel(); <-done }) }, nil
}
