// Command benchmark runs only against a fresh, isolated journey_benchmark DB.
// It uses real HTTP/transactions/workers and mock SMS; never SMTP or LLM APIs.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/analytics"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/delivery"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/httpapi"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/observability"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"golang.org/x/crypto/bcrypt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type distribution struct {
	Count int     `json:"count"`
	P50   float64 `json:"p50_ms"`
	P95   float64 `json:"p95_ms"`
	P99   float64 `json:"p99_ms"`
	Max   float64 `json:"max_ms"`
}

func percentiles(values []float64) distribution {
	sort.Float64s(values)
	at := func(q float64) float64 {
		if len(values) == 0 {
			return 0
		}
		position := float64(len(values)-1) * q
		low := int(math.Floor(position))
		high := int(math.Ceil(position))
		return values[low] + (values[high]-values[low])*(position-float64(low))
	}
	return distribution{len(values), at(.5), at(.95), at(.99), at(1)}
}

type peakPool struct {
	MaxAcquired int32 `json:"peak_acquired"`
	MaxTotal    int32 `json:"peak_total"`
	Limit       int32 `json:"connection_limit"`
	WaitCount   int64 `json:"empty_acquire_count"`
	WaitMS      int64 `json:"total_acquire_ms"`
}
type observation struct {
	mu         sync.Mutex
	Heap       uint64
	HeapSys    uint64
	Goroutines int
	Pools      map[string]peakPool
	MetricLag  float64
}

func (o *observation) sample(pools map[string]*pgxpool.Pool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	o.Heap = max(o.Heap, m.HeapAlloc)
	o.HeapSys = max(o.HeapSys, m.HeapSys)
	o.Goroutines = max(o.Goroutines, runtime.NumGoroutine())
	for name, p := range pools {
		s := p.Stat()
		v := o.Pools[name]
		v.MaxAcquired = max(v.MaxAcquired, s.AcquiredConns())
		v.MaxTotal = max(v.MaxTotal, s.TotalConns())
		v.Limit = s.MaxConns()
		v.WaitCount = s.EmptyAcquireCount()
		v.WaitMS = s.AcquireDuration().Milliseconds()
		o.Pools[name] = v
	}
}
func counter(ctx context.Context, pool *pgxpool.Pool, query string, args ...any) (int, error) {
	var n int
	err := pool.QueryRow(ctx, query, args...).Scan(&n)
	return n, err
}
func waitFor(ctx context.Context, label string, want int, pool *pgxpool.Pool, query string, args ...any) error {
	last := time.Time{}
	for {
		n, err := counter(ctx, pool, query, args...)
		if err != nil {
			return err
		}
		if n == want {
			return nil
		}
		if time.Since(last) >= 5*time.Second {
			fmt.Printf("%s: %d/%d\n", label, n, want)
			last = time.Now()
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s timed out", label)
		case <-time.After(time.Second):
		}
	}
}
func request(ctx context.Context, client *http.Client, base, path, method string, body any, token, csrf string) ([]byte, int, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, base+"/api/v1"+path, bytes.NewReader(raw))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	r, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer r.Body.Close()
	b, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	return b, r.StatusCode, err
}
func run(customers int, out string, rate, delayMS, delayEvery, spreadSeconds int, profile bool, observeSeconds, providerRate int, projectorMode string, dispatchers int, analyzeAt int, schedulers int, runtimeConcurrency int, dispatchBatch int) error {
	if projectorMode != "normal" && projectorMode != "paused" && projectorMode != "throttled" {
		return fmt.Errorf("unknown projector mode %q", projectorMode)
	}
	var projectorGate atomic.Int32
	if observeSeconds < 0 || observeSeconds > 300 {
		return fmt.Errorf("observation duration must be 0–300 seconds")
	}
	if rate < 0 || rate > 10000 || delayMS < 0 || delayMS > 10000 || delayEvery < 1 || spreadSeconds < 0 || spreadSeconds > 600 {
		return fmt.Errorf("invalid pacing/delay configuration")
	}
	if customers < 1 || customers > 100000 {
		return fmt.Errorf("customers must be 1–100,000")
	}
	dsn := os.Getenv("BENCH_DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("use make benchmark: isolated BENCH_DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	apiPool, err := database.Open(ctx, dsn)
	if err != nil {
		return err
	}
	defer apiPool.Close()
	var db string
	if err = apiPool.QueryRow(ctx, `SELECT current_database()`).Scan(&db); err != nil || db != "journey_benchmark" {
		return fmt.Errorf("benchmark refuses non-benchmark database")
	}
	if err = database.Migrate(ctx, apiPool); err != nil {
		return err
	}
	n, err := counter(ctx, apiPool, `SELECT count(*) FROM workspaces`)
	if err != nil || n != 0 {
		return fmt.Errorf("benchmark requires an empty isolated database")
	}
	if _, err = apiPool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pg_stat_statements`); err != nil {
		return err
	}
	if dispatchBatch < 1 || dispatchBatch > 64 {
		return fmt.Errorf("dispatch batch must be between 1 and 64")
	}
	if runtimeConcurrency < 1 || runtimeConcurrency > 64 {
		return fmt.Errorf("runtime concurrency must be between 1 and 64")
	}
	if schedulers < 1 || schedulers > 32 {
		return fmt.Errorf("schedulers must be between 1 and 32")
	}
	if dispatchers < 1 || dispatchers > 64 {
		return fmt.Errorf("dispatchers must be between 1 and 64")
	}
	// Runtime workers, delivery scheduling/recovery, the projector and probes share
	// this pool beside the dispatchers; keep the default size unless they need more.
	workerPoolMax := int32(max(20, runtimeConcurrency+dispatchers+schedulers+7))
	workerPool, err := database.OpenSized(ctx, dsn, workerPoolMax)
	if err != nil {
		return err
	}
	defer workerPool.Close()
	var workspace, user, journey, version string
	if err = apiPool.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES('Isolated benchmark','benchmark') RETURNING id::text`).Scan(&workspace); err != nil {
		return err
	}
	password := rand.Text()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}
	email := "benchmark@example.invalid"
	if err = apiPool.QueryRow(ctx, `INSERT INTO users(workspace_id,email,name,password_hash,role) VALUES($1,$2,'Benchmark operator',$3,'Admin') RETURNING id::text`, workspace, email, string(hash)).Scan(&user); err != nil {
		return err
	}
	spec := workflow.Workflow{SchemaVersion: 1, GlobalExit: &workflow.ExitCriteria{EventName: "purchase_completed"}, Nodes: []workflow.Node{{ID: "start", Type: "event_trigger", Label: "Cart", Config: workflow.Config{EventName: "cart_updated"}}, {ID: "delay", Type: "wait", Label: "Durable wait", Config: workflow.Config{DurationSeconds: 7200}}, {ID: "check", Type: "condition", Label: "Purchased?", Config: workflow.Config{Rule: &workflow.Rule{Path: "customer.attributes.purchased", Operator: "equals", Value: true}}}, {ID: "sms", Type: "sms", Label: "SMS", Config: workflow.Config{Text: "Your cart is waiting."}}, {ID: "done", Type: "exit", Label: "Complete", Config: workflow.Config{Reason: "completed"}}, {ID: "converted", Type: "exit", Label: "Converted", Config: workflow.Config{Reason: "converted"}}}, Edges: []workflow.Edge{{ID: "a", Source: "start", Target: "delay", Port: "started"}, {ID: "b", Source: "delay", Target: "check", Port: "completed"}, {ID: "c", Source: "check", Target: "converted", Port: "true"}, {ID: "d", Source: "check", Target: "sms", Port: "false"}, {ID: "e", Source: "sms", Target: "done", Port: "next"}}}
	if issues := workflow.Validate(spec); len(issues) > 0 {
		return fmt.Errorf("invalid benchmark graph")
	}
	flow, _ := json.Marshal(spec)
	if err = apiPool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,draft) VALUES($1,'Benchmark cart',$2) RETURNING id::text`, workspace, flow).Scan(&journey); err != nil {
		return err
	}
	if err = apiPool.QueryRow(ctx, `INSERT INTO journey_versions(workspace_id,journey_id,version,workflow,published_by) VALUES($1,$2,1,$3,$4) RETURNING id::text`, workspace, journey, flow, user).Scan(&version); err != nil {
		return err
	}
	if _, err = apiPool.Exec(ctx, `UPDATE journeys SET status='Active',entry_event_name='cart_updated',current_version_id=$2 WHERE id=$1`, journey, version); err != nil {
		return err
	}
	if _, err = apiPool.Exec(ctx, `INSERT INTO journeys(workspace_id,name) SELECT $1,'Other draft '||s FROM generate_series(1,1000)s`, workspace); err != nil {
		return err
	}
	// The SMS channel's provider rate: the product default unless the run raises it
	// to measure capacity rather than the configured provider limit.
	if providerRate > 0 {
		if _, err = apiPool.Exec(ctx, `INSERT INTO provider_channels(workspace_id,provider,rate_per_second) VALUES($1,'mock_sms',$2)`, workspace, providerRate); err != nil {
			return err
		}
	}
	if err = apiPool.QueryRow(ctx, `SELECT COALESCE((SELECT rate_per_second FROM provider_channels WHERE workspace_id=$1 AND provider='mock_sms'),(SELECT column_default::int FROM information_schema.columns WHERE table_name='provider_channels' AND column_name='rate_per_second'))`, workspace).Scan(&providerRate); err != nil {
		return err
	}
	if _, err = apiPool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,phone,attributes,consents) SELECT $1,'bench_'||lpad(s::text,6,'0'),'+12025550100','{"purchased":false}','{"email":false,"sms":true}' FROM generate_series(1,$2::int)s`, workspace, customers); err != nil {
		return err
	}
	token := "cjc_" + rand.Text()
	sum := sha256.Sum256([]byte(token))
	if _, err = apiPool.Exec(ctx, `INSERT INTO api_keys(workspace_id,created_by,name,token_hash,prefix) VALUES($1,$2,'Benchmark',$3,$4)`, workspace, user, hex.EncodeToString(sum[:]), token[:12]); err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	shutdownMetrics, err := observability.Serve(ctx, "127.0.0.1:0", workerPool, true, log)
	if err != nil {
		return err
	}
	defer shutdownMetrics()
	server := httptest.NewServer(httpapi.New(log, func(context.Context) error { return nil }, apiPool))
	defer server.Close()
	client := &http.Client{Timeout: 25 * time.Second, Transport: &http.Transport{MaxIdleConns: 64, MaxIdleConnsPerHost: 32}}
	jar, _ := cookiejar.New(nil)
	reader := &http.Client{Jar: jar, Timeout: 25 * time.Second}
	login, status, err := request(ctx, reader, server.URL, "/auth/login", "POST", map[string]string{"email": email, "password": password}, "", "")
	if err != nil || status != 200 {
		return fmt.Errorf("benchmark reader authentication failed")
	}
	var identity httpapi.Identity
	if json.Unmarshal(login, &identity) != nil {
		return fmt.Errorf("invalid benchmark reader identity")
	}
	if _, err = apiPool.Exec(ctx, `SELECT pg_stat_statements_reset()`); err != nil {
		return err
	}
	costAStart, err := costAt(ctx, apiPool)
	if err != nil {
		return err
	}
	probe := probeReads(ctx, reader, server.URL, journey)
	defer func() { probe.stop(); <-probe.done }()
	waits, stopWaits, err := startWaitProbe(ctx, dsn, func() string { return probe.phase.Load().(string) })
	if err != nil {
		return err
	}
	defer stopWaits()
	stopProfiles := func() error { return nil }
	if profile {
		stopProfiles, err = startProfiles(out)
		if err != nil {
			return err
		}
		defer func() { _ = stopProfiles() }()
	}
	workerCtx, stopWorkers := context.WithCancel(ctx)
	var workers sync.WaitGroup
	worker := engine.NewWorker(workerPool, nil)
	worker.Log = log
	worker.Concurrency = runtimeConcurrency
	sender := delivery.New(workerPool, nil)
	sender.Log = log
	sender.Dispatchers = dispatchers
	sender.Schedulers = schedulers
	sender.Batch = dispatchBatch
	sender.Providers["mock_sms"] = &delayedProvider{Provider: &delivery.Mock{Codes: []int{429, 500, 200}}, delay: time.Duration(delayMS) * time.Millisecond, every: int64(delayEvery)}
	workers.Go(func() { _ = worker.Run(workerCtx) })
	workers.Go(func() { _ = sender.Run(workerCtx) })
	if projectorMode == "normal" {
		workers.Go(func() { analytics.Run(workerCtx, workerPool, log) })
	} else {
		workers.Go(func() { runGatedProjector(workerCtx, workerPool, log, &projectorGate) })
	}
	defer func() { stopWorkers(); workers.Wait() }()
	observed := &observation{Pools: map[string]peakPool{}}
	projection := &projectionProbe{}
	monitorCtx, stopMonitor := context.WithCancel(ctx)
	var monitoring sync.WaitGroup
	monitoring.Go(func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		ticks := 0
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				observed.sample(map[string]*pgxpool.Pool{"api": apiPool, "worker": workerPool})
				ticks++
				if ticks%4 == 0 {
					projection.sample(monitorCtx, apiPool, probe.phase.Load().(string))
				}
			}
		}
	})
	defer func() { stopMonitor(); monitoring.Wait() }()
	events := make([]engine.Event, customers)
	for i := range events {
		events[i] = engine.Event{EventID: fmt.Sprintf("event_%06d", i+1), CustomerID: fmt.Sprintf("bench_%06d", i+1), EventName: "cart_updated", OccurredAt: time.Now().UTC(), Properties: map[string]any{"amount": 2450, "currency": "TRY"}}
	}
	latencies := make([]float64, customers)
	successfulLatencies := make([]float64, customers)
	var throttled atomic.Int64
	var accepted atomic.Int64
	errs := make(chan error, 20)
	jobs := make(chan int)
	var callers sync.WaitGroup
	started := time.Now()
	for range 20 {
		callers.Go(func() {
			for index := range jobs {
				begin := time.Now()
				for {
					attemptStarted := time.Now()
					b, status, e := request(ctx, client, server.URL, "/events", "POST", events[index], token, "")
					if e != nil {
						errs <- e
						return
					}
					if status == 429 {
						throttled.Add(1)
						select {
						case <-ctx.Done():
							errs <- ctx.Err()
							return
						case <-time.After(50 * time.Millisecond):
						}
						continue
					}
					if status != 200 {
						errs <- fmt.Errorf("ingestion returned HTTP %d", status)
						return
					}
					var result engine.Ingestion
					if json.Unmarshal(b, &result) != nil || len(result.Executions) != 1 {
						errs <- fmt.Errorf("logical enrollment missing")
						return
					}
					successfulLatencies[index] = float64(time.Since(attemptStarted).Microseconds()) / 1000
					latencies[index] = float64(time.Since(begin).Microseconds()) / 1000
					accepted.Add(1)
					break
				}
			}
		})
	}
	dispatchDone := make(chan struct{})
	go func() {
		defer close(dispatchDone)
		defer close(jobs)
		var pacing *time.Ticker
		if rate > 0 {
			pacing = time.NewTicker(time.Second / time.Duration(rate))
			defer pacing.Stop()
		}
		for i := range events {
			if pacing != nil {
				select {
				case <-pacing.C:
				case <-ctx.Done():
					return
				}
			}
			select {
			case jobs <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	callers.Wait()
	ingestElapsed := time.Since(started)
	select {
	case err := <-errs:
		return err
	default:
	}
	if accepted.Load() != int64(customers) {
		return fmt.Errorf("incomplete ingestion")
	}
	<-dispatchDone
	fmt.Printf("Ingested %d events in %.2fs\n", customers, ingestElapsed.Seconds())
	probe.phase.Store("runtime_to_wait")
	if err = waitFor(ctx, "Waiting executions", customers, apiPool, `SELECT count(*) FROM journey_executions WHERE workspace_id=$1 AND state='WAITING'`, workspace); err != nil {
		return err
	}
	waitingAt := time.Now()
	probe.phase.Store("waiting_steady")
	if observeSeconds > 0 {
		fmt.Printf("Observing %d active waits for %ds with authenticated journey reads\n", customers, observeSeconds)
		timer := time.NewTimer(time.Duration(observeSeconds) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	probe.phase.Store("waiting_drain")
	costAEnd, err := costAt(ctx, apiPool)
	if err != nil {
		return err
	}
	stageAQueries, err := queryStats(ctx, apiPool)
	if err != nil {
		return err
	}
	active, err := counter(ctx, apiPool, `SELECT count(*) FROM journey_executions WHERE workspace_id=$1 AND state IN ('RUNNING','WAITING')`, workspace)
	if err != nil {
		return err
	}
	for i := 0; i < min(100, customers); i++ {
		b, status, e := request(ctx, client, server.URL, "/events", "POST", events[i], token, "")
		if e != nil || status != 200 {
			return fmt.Errorf("duplicate request failed")
		}
		var result engine.Ingestion
		json.Unmarshal(b, &result)
		if result.Status != "DUPLICATE" || len(result.Executions) != 0 {
			return fmt.Errorf("duplicate enrollment")
		}
	}
	if _, err = apiPool.Exec(ctx, `ANALYZE`); err != nil {
		return err
	}
	if _, err = apiPool.Exec(ctx, `SELECT pg_stat_statements_reset()`); err != nil {
		return err
	}
	costBStart, err := costAt(ctx, apiPool)
	if err != nil {
		return err
	}
	probe.phase.Store("timer_burst")
	// The projector mode applies from the burst until every message is accepted.
	switch projectorMode {
	case "paused":
		projectorGate.Store(gatePaused)
	case "throttled":
		projectorGate.Store(gateThrottled)
	}
	due := time.Now().UTC().Add(5 * time.Second)
	// With -analyze-delivery-at, delivery_jobs is analyzed once the burst has
	// queued that many messages, standing in for a table whose statistics already
	// describe its contents, as a long-running deployment's would.
	analyzedAt := atomic.Int64{}
	analyzedAt.Store(-1)
	if analyzeAt > 0 {
		monitoring := sync.WaitGroup{}
		analyzeCtx, stopAnalyze := context.WithCancel(ctx)
		defer func() { stopAnalyze(); monitoring.Wait() }()
		monitoring.Go(func() {
			for analyzeCtx.Err() == nil {
				var queued int
				if err := apiPool.QueryRow(analyzeCtx, `SELECT count(*) FROM delivery_jobs`).Scan(&queued); err == nil && queued >= analyzeAt {
					if _, err := apiPool.Exec(analyzeCtx, `ANALYZE delivery_jobs`); err == nil {
						analyzedAt.Store(time.Since(due).Milliseconds())
					}
					return
				}
				select {
				case <-analyzeCtx.Done():
				case <-time.After(200 * time.Millisecond):
				}
			}
		})
	}
	// One sample a second of where stage B's work sits: runtime completions,
	// actions waiting to be scheduled, and messages queued, in flight and accepted.
	var timeline []map[string]any
	var timelineMu sync.Mutex
	timelineCtx, stopTimeline := context.WithCancel(ctx)
	timelineDone := make(chan struct{})
	go func() {
		defer close(timelineDone)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-timelineCtx.Done():
				return
			case <-ticker.C:
			}
			var completed, pending, scheduled, inFlight, accepted int
			if err := apiPool.QueryRow(timelineCtx, `SELECT (SELECT count(*) FROM journey_executions WHERE workspace_id=$1 AND state='COMPLETED'),(SELECT count(*) FROM action_outbox WHERE workspace_id=$1 AND state='PENDING'),count(*) FILTER (WHERE status='SCHEDULED'),count(*) FILTER (WHERE status IN ('CLAIMED','DISPATCHING')),count(*) FILTER (WHERE status='ACCEPTED') FROM delivery_jobs WHERE workspace_id=$1`, workspace).Scan(&completed, &pending, &scheduled, &inFlight, &accepted); err != nil {
				continue
			}
			timelineMu.Lock()
			timeline = append(timeline, map[string]any{"s": time.Since(due).Seconds(), "executions_completed": completed, "outbox_pending": pending, "delivery_scheduled": scheduled, "delivery_in_flight": inFlight, "delivery_accepted": accepted})
			timelineMu.Unlock()
		}
	}()
	stopTimelineOnce := sync.OnceFunc(func() { stopTimeline(); <-timelineDone })
	defer stopTimelineOnce()
	if spreadSeconds == 0 {
		if _, err = apiPool.Exec(ctx, `UPDATE scheduled_jobs SET due_at=$2 WHERE workspace_id=$1 AND kind='timer' AND state='SCHEDULED'`, workspace, due); err != nil {
			return err
		}
	} else if _, err = apiPool.Exec(ctx, timerSpreadUpdate, workspace, due, spreadSeconds); err != nil {
		return err
	}
	lastDue := due
	if customers > 1 {
		lastDue = due.Add(time.Duration(spreadSeconds) * time.Second)
	}
	if _, err = apiPool.Exec(ctx, `ANALYZE scheduled_jobs`); err != nil {
		return err
	}
	plans := map[string]json.RawMessage{}
	explain := func(name, query string, args ...any) error {
		var raw []byte
		if e := apiPool.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+query, args...).Scan(&raw); e != nil {
			return fmt.Errorf("plan %s: %w", name, e)
		}
		plans[name] = raw
		return nil
	}
	if err = explain("joined_due_job_reference", `SELECT j.id FROM scheduled_jobs j JOIN journey_executions e ON e.id=j.execution_id JOIN journeys f ON f.id=e.journey_id WHERE j.state='SCHEDULED' AND j.due_at<=$1 AND e.state IN ('RUNNING','WAITING') AND f.status IN ('Active','Sunset') ORDER BY j.due_at,j.id LIMIT 32 FOR UPDATE OF j SKIP LOCKED`, lastDue.Add(time.Second)); err != nil {
		return err
	}
	if err = explain("loaded_due_job_claim", orderedClaimCandidate, lastDue.Add(time.Second)); err != nil {
		return err
	}
	if err = explain("loaded_outbox_discovery", delivery.PendingPageQuery, 32); err != nil {
		return err
	}
	if err = explain("active_entry_matching", `SELECT id,current_version_id FROM journeys WHERE workspace_id=$1 AND status='Active' AND entry_event_name='cart_updated'`, workspace); err != nil {
		return err
	}
	if err = explain("customer_tracking", `SELECT id,state,current_node_id FROM journey_executions WHERE workspace_id=$1 AND journey_id=$2 AND customer_id='bench_000001' ORDER BY started_at DESC,id DESC LIMIT 50`, workspace, journey); err != nil {
		return err
	}
	if err = explain("event_execution_lookup", `SELECT id,journey_id,version_id,customer_id,entry_event_id,state,exit_reason,started_at FROM journey_executions WHERE workspace_id=$1 AND entry_event_id=$2 ORDER BY started_at DESC,id LIMIT 25`, workspace, events[0].EventID); err != nil {
		return err
	}
	if err = waitFor(ctx, "Completed executions", customers, apiPool, `SELECT count(*) FROM journey_executions WHERE workspace_id=$1 AND state='COMPLETED'`, workspace); err != nil {
		return err
	}
	runtimeDone := time.Now()
	probe.phase.Store("delivery_drain")
	held, err := delivery.HeldChannels(ctx, apiPool, time.Now().UTC())
	if err != nil {
		return err
	}
	if err = explain("loaded_delivery_head_claim", delivery.ClaimLiveHead, time.Now().UTC()); err != nil {
		return err
	}
	if err = explain("loaded_delivery_claim", delivery.ClaimLive, time.Now().UTC(), held); err != nil {
		return err
	}
	if err = waitFor(ctx, "Accepted mock deliveries", customers, apiPool, `SELECT count(*) FROM delivery_jobs WHERE workspace_id=$1 AND status='ACCEPTED'`, workspace); err != nil {
		return err
	}
	deliveryDone := time.Now()
	stopTimelineOnce()
	costAccepted, err := costAt(ctx, apiPool)
	if err != nil {
		return err
	}
	var queuedMetrics int64
	if err = apiPool.QueryRow(ctx, `SELECT count(*) FROM metric_queue`).Scan(&queuedMetrics); err != nil {
		return err
	}
	projectorGate.Store(gateOpen)
	if err = explainFence(ctx, apiPool, workspace, plans); err != nil {
		return err
	}
	probe.phase.Store("projection_catchup")
	if err = waitFor(ctx, "Metric queue drain", 0, apiPool, `SELECT (SELECT count(*) FROM metric_queue)+(SELECT count(*) FROM metric_snapshot_backfill WHERE NOT complete)`); err != nil {
		return err
	}
	projectionCaughtUp := time.Now()
	loadedReads, readFailures := probe.finish()
	if len(readFailures) > 0 {
		return fmt.Errorf("loaded read requests failed: %v", readFailures)
	}
	cohortCheck, err := verifyJourneyObservation(ctx, reader, server.URL, journey, customers)
	if err != nil {
		return err
	}
	costBEnd, err := costAt(ctx, apiPool)
	if err != nil {
		return err
	}
	// Trigger and function time, recorded only when the server tracks functions.
	functionTime := map[string]map[string]float64{}
	fnRows, err := apiPool.Query(ctx, `SELECT funcname,calls,total_time,self_time FROM pg_stat_user_functions`)
	if err != nil {
		return err
	}
	for fnRows.Next() {
		var name string
		var calls int64
		var total, self float64
		if err = fnRows.Scan(&name, &calls, &total, &self); err != nil {
			fnRows.Close()
			return err
		}
		functionTime[name] = map[string]float64{"calls": float64(calls), "total_ms": total, "self_ms": self}
	}
	fnRows.Close()
	tableScans := map[string]map[string]int64{}
	scanRows, err := apiPool.Query(ctx, `SELECT relname,seq_scan,seq_tup_read,COALESCE(idx_scan,0),COALESCE(idx_tup_fetch,0),n_tup_upd,n_tup_hot_upd,n_dead_tup,autoanalyze_count,COALESCE((EXTRACT(epoch FROM last_autoanalyze-$1)*1000)::bigint,-1) FROM pg_stat_user_tables WHERE relname IN ('action_outbox','delivery_jobs','delivery_attempts','scheduled_jobs','journey_executions','step_executions','customers','metric_queue','provider_channels')`, due)
	if err != nil {
		return err
	}
	for scanRows.Next() {
		var name string
		var seq, seqRead, idx, idxFetch, upd, hot, dead, analyzes, lastAnalyze int64
		if err = scanRows.Scan(&name, &seq, &seqRead, &idx, &idxFetch, &upd, &hot, &dead, &analyzes, &lastAnalyze); err != nil {
			scanRows.Close()
			return err
		}
		tableScans[name] = map[string]int64{"seq_scan": seq, "seq_tup_read": seqRead, "idx_scan": idx, "idx_tup_fetch": idxFetch, "updates": upd, "hot_updates": hot, "dead_tuples": dead, "autoanalyze_count": analyzes, "last_autoanalyze_ms_after_stage_b_due": lastAnalyze}
	}
	scanRows.Close()
	stageBQueries, err := queryStats(ctx, apiPool)
	if err != nil {
		return err
	}
	if _, err = apiPool.Exec(ctx, `ANALYZE`); err != nil {
		return err
	}
	for name, q := range map[string]string{
		"workspace_summary":      `SELECT dimension,dimension_key,metric,value FROM metric_totals WHERE workspace_id=$1 AND journey_id='00000000-0000-0000-0000-000000000000' AND version_id='00000000-0000-0000-0000-000000000000' AND dimension<>'node'`,
		"hourly_trend":           `SELECT bucket::date,metric,sum(value) FROM metric_hourly WHERE workspace_id=$1 AND journey_id='00000000-0000-0000-0000-000000000000' AND version_id='00000000-0000-0000-0000-000000000000' AND bucket>=now()-interval '30 days' AND dimension='execution' GROUP BY 1,2`,
		"journey_prefix_page":    `SELECT id,name,status,updated_at FROM journeys WHERE workspace_id=$1 AND lower(name) COLLATE "C" LIKE 'bench%' ORDER BY updated_at DESC,id DESC LIMIT 50`,
		"recent_execution_page":  `SELECT id,state,started_at FROM journey_executions WHERE workspace_id=$1 ORDER BY started_at DESC,id DESC LIMIT 50`,
		"frequency_reservations": `SELECT count(*) FROM (SELECT 1 FROM ((SELECT 1 FROM delivery_jobs WHERE workspace_id=$1 AND customer_id='bench_000001' AND mode='LIVE' AND status IN ('SCHEDULED','CLAIMED','DISPATCHING','UNKNOWN') LIMIT 3) UNION ALL (SELECT 1 FROM delivery_jobs WHERE workspace_id=$1 AND customer_id='bench_000001' AND mode='LIVE' AND status='ACCEPTED' AND accepted_at>=now()-interval '1 day' LIMIT 3))s LIMIT 3)r`,
	} {
		if err = explain(name, q, workspace); err != nil {
			return err
		}
	}
	var lag []float64
	if err = apiPool.QueryRow(ctx, `SELECT percentile_cont(ARRAY[0.5,0.95,0.99,1.0]) WITHIN GROUP(ORDER BY extract(epoch FROM s.completed_at-j.due_at)*1000) FROM scheduled_jobs j JOIN step_executions s ON s.execution_id=j.execution_id AND s.node_id=j.node_id WHERE j.workspace_id=$1 AND j.kind='timer'`, workspace).Scan(&lag); err != nil {
		return err
	}
	var eventCount, executionCount, logical, attempts, jobRetries, deliveryRetries, failures, duplicates int
	for _, q := range []struct {
		sql  string
		dest *int
	}{
		{`SELECT count(*) FROM customer_events WHERE workspace_id=$1`, &eventCount}, {`SELECT count(*) FROM journey_executions WHERE workspace_id=$1`, &executionCount}, {`SELECT count(*) FROM delivery_jobs WHERE workspace_id=$1`, &logical}, {`SELECT count(*) FROM delivery_attempts WHERE workspace_id=$1`, &attempts}, {`SELECT COALESCE(sum(greatest(attempts-1,0)),0) FROM scheduled_jobs WHERE workspace_id=$1`, &jobRetries}, {`SELECT COALESCE(sum(greatest(attempts-1,0)),0) FROM delivery_jobs WHERE workspace_id=$1`, &deliveryRetries}, {`SELECT count(*) FROM journey_executions WHERE workspace_id=$1 AND state='FAILED'`, &failures}, {`SELECT count(*)-count(DISTINCT (execution_id,node_id)) FROM delivery_jobs WHERE workspace_id=$1`, &duplicates},
	} {
		if err = apiPool.QueryRow(ctx, q.sql, workspace).Scan(q.dest); err != nil {
			return err
		}
	}
	if executionCount != customers || eventCount != customers || logical != customers || failures != 0 || duplicates != 0 || active != customers {
		return fmt.Errorf("benchmark consistency invariant failed")
	}
	readLatency := map[string]distribution{}
	for _, route := range observationRoutes(journey) {
		name, path := route.name, route.path
		samples := []float64{}
		for range 30 {
			begin := time.Now()
			_, code, e := request(ctx, reader, server.URL, path, "GET", nil, "", identity.CSRF)
			if e != nil || code != 200 {
				return fmt.Errorf("read benchmark %s failed with HTTP %d", name, code)
			}
			samples = append(samples, float64(time.Since(begin).Microseconds())/1000)
			// Keep warm probes below the normal workspace UI rate limit.
			time.Sleep(110 * time.Millisecond)
		}
		readLatency[name] = percentiles(samples)
	}
	observed.sample(map[string]*pgxpool.Pool{"api": apiPool, "worker": workerPool})
	stopMonitor()
	monitoring.Wait()
	var postgres string
	if err = apiPool.QueryRow(ctx, `SELECT version()`).Scan(&postgres); err != nil {
		return err
	}
	report := map[string]any{"recorded_at": time.Now().UTC(), "commit": os.Getenv("BENCH_COMMIT"), "working_tree_dirty": os.Getenv("BENCH_DIRTY") == "true", "customers": customers, "mvp_load_threshold_met": customers >= 10000, "offered_events_per_second": rate, "timer_schedule": map[string]any{"spread_seconds": spreadSeconds, "first_due_at": due, "last_due_at": lastDue}, "fault_configuration": map[string]int{"provider_delay_ms": delayMS, "delay_every_attempts": delayEvery}, "provider_rate_per_second": providerRate, "projector_mode": projectorMode, "stage_b_pipeline_timeline": timeline, "delivery_analyze": map[string]any{"at_queued_messages": analyzeAt, "ms_after_stage_b_due": analyzedAt.Load()}, "telemetry": map[string]any{"metrics_enabled": true, "queue_probe_interval_seconds": 5, "trace_export_enabled": false}, "environment": map[string]any{"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "logical_cpu_count": runtime.NumCPU(), "postgres": postgres, "postgres_container_cpu_limit": envInt("BENCH_PG_CPUS", 2), "postgres_container_memory_limit_bytes": envInt("BENCH_PG_MEMORY_BYTES", 2<<30), "http_concurrency": 20, "runtime_concurrency": runtimeConcurrency, "delivery_concurrency": dispatchers, "delivery_schedulers": schedulers, "delivery_batch": dispatchBatch, "pool_max_per_process": map[string]int32{"api": apiPool.Config().MaxConns, "worker": workerPoolMax}}, "method": "Real loopback HTTP ingestion and actual PostgreSQL runtime/delivery/projector, warm local Docker database. Fixture creates 1,000 inactive draft journeys and synthetic customers. Stage A reaches the requested number of simultaneously WAITING durable executions (MVP threshold: 10,000). Stage B moves only isolated fixture timer deadlines to a declared burst or spread beginning five seconds ahead, then lets real workers process. No frozen clock, SMTP or LLM requests. Completion polling interval is 1s; Go memory includes benchmark client+API+worker in one process. Two transient mock provider outcomes (429/500) are injected. Up to five serial authenticated UI reads/sec rotate across nine routes, including journey insights and drilldowns, throughout loaded phases; requests are not an open-loop saturation test. pg_stat_statements records top-level statement execution (including trigger cost); client network/round-trip time is not included. Timer lateness measures step completion after due_at, not claim time. EXPLAIN probes do not modify job state. Metrics/SQL timing and bounded queue sampling are enabled; no trace exporter is started. Optional pacing and provider delay are declared in the report. Successful-attempt latency excludes earlier 429 waits; logical latency includes them.", "ingestion": map[string]any{"seconds": ingestElapsed.Seconds(), "events_per_second": float64(customers) / ingestElapsed.Seconds(), "event_to_committed_execution_upper_bound": percentiles(latencies), "successful_http_attempt_latency": percentiles(successfulLatencies), "http_429_retries": throttled.Load(), "duplicate_requests": min(100, customers)}, "database_queries": map[string]any{"stage_a": stageAQueries, "stage_b": stageBQueries}, "loaded_read_http_latency": loadedReads, "loaded_read_failures": readFailures, "runtime": map[string]any{"peak_confirmed_active": active, "stage_a_until_all_waiting_seconds": waitingAt.Sub(started).Seconds(), "stage_a_jobs_per_second": float64(2*customers) / waitingAt.Sub(started).Seconds(), "stage_b_due_at": due, "stage_b_until_all_completed_seconds": runtimeDone.Sub(due).Seconds(), "stage_b_jobs_per_second": float64(4*customers) / runtimeDone.Sub(due).Seconds(), "pipeline_until_all_provider_accepted_seconds": deliveryDone.Sub(due).Seconds(), "timer_step_completion_lateness_ms": map[string]any{"p50": lag[0], "p95": lag[1], "p99": lag[2], "max": lag[3]}}, "consistency": map[string]any{"events": eventCount, "executions": executionCount, "logical_deliveries": logical, "provider_attempts": attempts, "runtime_retry_attempts": jobRetries, "delivery_retry_attempts": deliveryRetries, "failed_executions": failures, "duplicate_logical_actions": duplicates}, "read_http_latency": readLatency, "memory": map[string]any{"go_peak_heap_alloc_bytes": observed.Heap, "go_peak_heap_sys_bytes": observed.HeapSys, "peak_goroutines": observed.Goroutines}, "pools": observed.Pools, "query_plans": plans, "table_activity": tableScans, "function_time": functionTime, "work_cost": map[string]any{"meaning": "PostgreSQL work per 1,000 executions that finished the stage (stage A: reached the durable wait; stage B: completed with the message accepted and projections caught up). CPU is the database container's cgroup usage, so it holds regardless of what else runs on the host. SQL figures cover top-level statements, including the benchmark's own reads, polling and EXPLAIN probes.", "stage_a": costAEnd.since(costAStart, customers), "stage_b": costBEnd.since(costBStart, customers), "stage_b_until_accepted": costAccepted.since(costBStart, customers), "projection_catchup": costBEnd.since(costAccepted, customers), "metric_queue_rows_when_accepted": queuedMetrics}}
	stopWaits()
	report["database_wait_sampling"] = waits.report()
	report["journey_observability"] = map[string]any{
		"read_routes":                               observationRoutesReport(journey),
		"steady_wait_observation_seconds":           observeSeconds,
		"projection_catchup_after_delivery_seconds": projectionCaughtUp.Sub(deliveryDone).Seconds(),
		"projection_sampling":                       projection,
		"reconciliation":                            cohortCheck,
	}
	if err = addObservationPlans(ctx, apiPool, workspace, journey, version, explain, report); err != nil {
		return err
	}
	report["go_profiling_enabled"] = profile
	if err = stopProfiles(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	if err = os.WriteFile(out, append(data, '\n'), 0644); err != nil {
		return err
	}
	recorder := httptest.NewRecorder()
	observability.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	if err = os.WriteFile(out+".prom", recorder.Body.Bytes(), 0644); err != nil {
		return err
	}
	fmt.Printf("PASS: %d active executions; %d logical mock sends; %d duplicate actions. Report: %s\n", active, logical, duplicates, out)
	return nil
}
func main() {
	customers := flag.Int("customers", 10000, "Synthetic customers (10,000 for MVP acceptance)")
	out := flag.String("out", "benchmark.json", "Report output")
	rate := flag.Int("ingest-rate", 0, "Offered single events/sec; zero is unpaced")
	delay := flag.Int("provider-delay-ms", 0, "Delay injected into every Nth mock provider attempt")
	every := flag.Int("provider-delay-every", 4, "Mock attempts per injected delay")
	spread := flag.Int("timer-spread-seconds", 0, "Spread fixture timer deadlines across 0–600 seconds; zero keeps the synchronized burst")
	observe := flag.Int("observe-wait-seconds", 0, "Hold all executions at their durable wait for 0–300 seconds of journey API observation")
	providerRate := flag.Int("provider-rate", 0, "SMS provider calls/sec for the benchmark workspace; zero keeps the product default (50)")
	projector := flag.String("projector", "normal", "Metric projector from the timer burst until every message is accepted: normal, paused (no draining) or throttled (one batch every 5 s while runtime or delivery work is due)")
	analyzeAt := flag.Int("analyze-delivery-at", 0, "ANALYZE delivery_jobs once the burst has queued this many messages; zero leaves statistics to autovacuum")
	dispatchBatch := flag.Int("dispatch-batch", 1, "Messages a dispatcher claims and sends together on a healthy channel (product default 1)")
	runtimeConcurrency := flag.Int("runtime-concurrency", 8, "Runtime jobs processed at once (product default 8)")
	schedulers := flag.Int("schedulers", 1, "Delivery scheduling loops turning pending actions into messages (product default 1)")
	dispatchers := flag.Int("dispatchers", 4, "Delivery claim-and-send loops (product default 4); the worker pool grows past 20 connections when they need it")
	profile := flag.Bool("profile", false, "Write CPU and allocation profiles beside report (synthetic process includes client/API/workers)")
	flag.Parse()
	if err := run(*customers, *out, *rate, *delay, *every, *spread, *profile, *observe, *providerRate, *projector, *dispatchers, *analyzeAt, *schedulers, *runtimeConcurrency, *dispatchBatch); err != nil {
		fmt.Fprintln(os.Stderr, "Benchmark failed:", err)
		os.Exit(1)
	}
}

// costPoint is a reading of how much work PostgreSQL has done so far.
type costPoint struct {
	at        time.Time
	cpuMicros *int64
	walBytes  int64
	calls     int64
	execMs    float64
	blocks    int64
	// goCPU is this process's user+system CPU: client, API and workers.
	goCPU time.Duration
}

func processCPU() time.Duration {
	var u syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &u) != nil {
		return 0
	}
	return time.Duration(u.Utime.Nano() + u.Stime.Nano())
}

// costAt reads the database container's CPU use, the WAL position and the
// statement totals. CPU is unknown where the cgroup file cannot be read.
func costAt(ctx context.Context, pool *pgxpool.Pool) (costPoint, error) {
	p := costPoint{at: time.Now(), goCPU: processCPU()}
	var cpu int64
	if pool.QueryRow(ctx, `SELECT (regexp_match(pg_read_file('/sys/fs/cgroup/cpu.stat'),'usage_usec (\d+)'))[1]::bigint`).Scan(&cpu) == nil {
		p.cpuMicros = &cpu
	}
	err := pool.QueryRow(ctx, `SELECT pg_wal_lsn_diff(pg_current_wal_lsn(),'0/0')::bigint,COALESCE(sum(calls),0)::bigint,COALESCE(sum(total_exec_time),0),COALESCE(sum(shared_blks_hit+shared_blks_read),0)::bigint FROM pg_stat_statements`).Scan(&p.walBytes, &p.calls, &p.execMs, &p.blocks)
	return p, err
}

// since is the work done between start and p, per 1,000 executions.
func (p costPoint) since(start costPoint, executions int) map[string]any {
	per := 1000 / float64(executions)
	seconds := p.at.Sub(start.at).Seconds()
	out := map[string]any{
		"wall_seconds":                 seconds,
		"wal_mib_per_1000":             float64(p.walBytes-start.walBytes) / (1 << 20) * per,
		"sql_calls_per_1000":           float64(p.calls-start.calls) * per,
		"sql_exec_seconds_per_1000":    (p.execMs - start.execMs) / 1000 * per,
		"shared_blocks_per_1000":       float64(p.blocks-start.blocks) * per,
		"go_process_cpu_cores_average": (p.goCPU - start.goCPU).Seconds() / seconds,
	}
	if p.cpuMicros != nil && start.cpuMicros != nil {
		cpu := float64(*p.cpuMicros-*start.cpuMicros) / 1e6
		out["db_cpu_seconds_per_1000"] = cpu * per
		out["db_cpu_cores_average"] = cpu / seconds
	}
	return out
}

// explainFence plans the scheduling fence, custom and generic, against one of
// this run's actions put back to pending inside a transaction that is rolled
// back, so the frequency reservation count runs as it did under load.
func explainFence(ctx context.Context, pool *pgxpool.Pool, workspace string, plans map[string]json.RawMessage) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var outbox, execution, customer string
	if err = tx.QueryRow(ctx, `SELECT o.id::text,o.execution_id::text,e.customer_id FROM action_outbox o JOIN journey_executions e ON e.id=o.execution_id WHERE o.workspace_id=$1 ORDER BY o.created_at DESC LIMIT 1`, workspace).Scan(&outbox, &execution, &customer); err != nil {
		return fmt.Errorf("fence sample: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE action_outbox SET state='PENDING' WHERE id=$1`, outbox); err != nil {
		return err
	}
	for _, mode := range []string{"force_custom_plan", "force_generic_plan"} {
		if _, err = tx.Exec(ctx, `SET LOCAL plan_cache_mode=`+mode); err != nil {
			return err
		}
		var raw []byte
		if err = tx.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+delivery.ScheduleFenceQuery, execution, outbox, workspace, customer, time.Now().UTC().Add(-24*time.Hour), true, 3).Scan(&raw); err != nil {
			return fmt.Errorf("plan fence: %w", err)
		}
		plans["loaded_schedule_fence_"+mode] = raw
	}
	return nil
}

const (
	gateOpen int32 = iota
	gatePaused
	gateThrottled
)

// runGatedProjector drains the metric queue like analytics.Run while the gate is
// open. Paused, it drains nothing; throttled, it drains one batch every five
// seconds while timer or delivery work is due, and normally otherwise.
func runGatedProjector(ctx context.Context, pool *pgxpool.Pool, log *slog.Logger, gate *atomic.Int32) {
	sleep := func(d time.Duration) bool {
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return true
		}
	}
	for ctx.Err() == nil {
		switch gate.Load() {
		case gatePaused:
			if !sleep(250 * time.Millisecond) {
				return
			}
			continue
		case gateThrottled:
			var busy bool
			if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM scheduled_jobs WHERE state='SCHEDULED' AND due_at<=now()) OR EXISTS(SELECT 1 FROM delivery_jobs WHERE status='SCHEDULED' AND due_at<=now())`).Scan(&busy); err == nil && busy {
				if _, err := analytics.Drain(ctx, pool); err != nil && ctx.Err() == nil {
					log.Error("analytics projection failed")
				}
				if !sleep(5 * time.Second) {
					return
				}
				continue
			}
		}
		n, err := analytics.Drain(ctx, pool)
		if err != nil && ctx.Err() == nil {
			log.Error("analytics projection failed")
		}
		if err == nil && n == 1000 {
			continue
		}
		if !sleep(time.Second) {
			return
		}
	}
}

// envInt reads a numeric setting the benchmark script declares, such as the
// database container's CPU limit, so the report matches the run.
func envInt(name string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return fallback
}
