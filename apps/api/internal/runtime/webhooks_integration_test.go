//go:build integration

package runtime

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

func TestWebhookStepRecordsOnceAndSendsOnlyToAllowedHosts(t *testing.T) {
	ctx, pool, workspace, journey, _, spec := fixture(t)
	clock := &testClock{now: time.Now().UTC()}
	var mu sync.Mutex
	var bodies []map[string]any
	var keys []string
	status := http.StatusInternalServerError
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		bodies = append(bodies, body)
		keys = append(keys, r.Header.Get("Idempotency-Key"))
		code := status
		mu.Unlock()
		w.WriteHeader(code)
	}))
	defer receiver.Close()
	target, _ := url.Parse(receiver.URL)

	spec.Nodes = append(spec.Nodes,
		workflow.Node{ID: "notify", Type: "webhook", Label: "Tell the CRM", Config: workflow.Config{URL: receiver.URL + "/hook", Fields: map[string]string{"tier": "customer.attributes.tier", "missing": "customer.attributes.absent"}}},
		workflow.Node{ID: "elsewhere", Type: "webhook", Label: "Unlisted host", Config: workflow.Config{URL: "https://not-allowed.example.invalid/hook"}},
	)
	spec.Edges = []workflow.Edge{{ID: "a", Source: "start", Target: "notify", Port: "started"}, {ID: "b", Source: "notify", Target: "elsewhere", Port: "next"}, {ID: "c", Source: "elsewhere", Target: "done", Port: "next"}}
	if issues := workflow.Validate(spec); len(issues) != 0 {
		t.Fatal(issues)
	}
	publishTest(t, ctx, pool, workspace, journey, spec)
	if _, err := pool.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,attributes) VALUES($1,'hook_customer','{"tier":"gold"}') ON CONFLICT (workspace_id,customer_id) DO UPDATE SET attributes=excluded.attributes`, workspace); err != nil {
		t.Fatal(err)
	}
	execution := entry(t, ctx, pool, workspace, "hook_customer", clock.Now())
	worker := NewWorker(pool, clock)
	drain(t, ctx, worker)
	// The journey does not wait for the receiver.
	if state := executionState(t, ctx, pool, execution); state != "COMPLETED" {
		t.Fatal("webhook steps must not hold the journey", state)
	}
	var recorded int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_calls WHERE execution_id=$1 AND state='PENDING'`, execution).Scan(&recorded); err != nil || recorded != 2 {
		t.Fatal("each webhook step records exactly one call", recorded, err)
	}

	dispatcher := NewWebhookDispatcher(pool, AllowedWebhookHosts(" "+target.Host+" ,"))
	run := func() {
		for {
			handled, err := dispatcher.Dispatch(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if !handled {
				return
			}
		}
	}
	state := func(node string) (string, int, *int) {
		var value string
		var attempts int
		var code *int
		if err := pool.QueryRow(ctx, `SELECT state,attempts,response_status FROM webhook_calls WHERE execution_id=$1 AND node_id=$2`, execution, node).Scan(&value, &attempts, &code); err != nil {
			t.Fatal(err)
		}
		return value, attempts, code
	}
	// The receiver fails every time: the call is retried and then gives up.
	run()
	if value, attempts, code := state("notify"); value != "FAILED" || attempts != webhookAttempts || code == nil || *code != 500 {
		t.Fatal("a failing receiver is retried, then marked failed", value, attempts, code)
	}
	// A host that is not on the allowlist never receives anything.
	if value, attempts, code := state("elsewhere"); value != "RECORDED" || attempts != 1 || code != nil {
		t.Fatal("an unlisted host must only be recorded", value, attempts, code)
	}
	mu.Lock()
	if len(bodies) != webhookAttempts {
		t.Fatal("only the allowed host is contacted", len(bodies))
	}
	body := bodies[0]
	for _, key := range keys {
		if key == "" || key != keys[0] {
			t.Fatal("every attempt carries the same idempotency key", keys)
		}
	}
	mu.Unlock()
	data, _ := body["data"].(map[string]any)
	if body["customer_id"] != "hook_customer" || body["execution_id"] != execution || body["step"] != "notify" || data["tier"] != "gold" {
		t.Fatal("unexpected payload", body)
	}
	if value, present := data["missing"]; !present || value != nil {
		t.Fatal("a field without a value is sent as null", data)
	}

	// A healthy receiver gets the call once.
	mu.Lock()
	status = http.StatusNoContent
	bodies = nil
	mu.Unlock()
	if _, err := pool.Exec(ctx, `UPDATE webhook_calls SET state='PENDING',attempts=0 WHERE execution_id=$1 AND node_id='notify'`, execution); err != nil {
		t.Fatal(err)
	}
	run()
	run()
	if value, attempts, code := state("notify"); value != "SENT" || attempts != 1 || code == nil || *code != 204 {
		t.Fatal("a successful call is sent once", value, attempts, code)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 {
		t.Fatal("a sent call is not repeated", len(bodies))
	}
	// Replaying the step cannot create a second logical call.
	if _, err := pool.Exec(context.Background(), `INSERT INTO webhook_calls(workspace_id,execution_id,node_id,customer_id,url,payload,created_at) VALUES($1,$2,'notify','hook_customer','https://x.invalid','{}',now()) ON CONFLICT DO NOTHING`, workspace, execution); err != nil {
		t.Fatal(err)
	}
	var total int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_calls WHERE execution_id=$1`, execution).Scan(&total); err != nil || total != 2 {
		t.Fatal("one call per execution and step", total, err)
	}
}
