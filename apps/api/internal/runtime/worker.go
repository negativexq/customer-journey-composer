package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ofk/customer-journey-composer/apps/api/internal/observability"
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
	"go.opentelemetry.io/otel/attribute"
)

const maxTurnNodes = 8
const maxTurnDuration = 10 * time.Millisecond

type Worker struct {
	Pool  *pgxpool.Pool
	Clock Clock
	Log   *slog.Logger
}
type Job struct {
	ID, WorkspaceID, ExecutionID, CustomerID, JourneyID, NodeID, Kind, Token, TraceParent string
	Attempts                                                                              int
	DueAt                                                                                 time.Time
}

func NewWorker(pool *pgxpool.Pool, clock Clock) *Worker {
	if clock == nil {
		clock = RealClock{}
	}
	return &Worker{Pool: pool, Clock: clock, Log: slog.Default()}
}
func (w *Worker) Claim(ctx context.Context, limit int) ([]Job, error) {
	if err := w.recoverLeases(ctx); err != nil {
		return nil, err
	}
	return w.claimReady(ctx, limit)
}
func (w *Worker) recoverLeases(ctx context.Context) error {
	tag, err := w.Pool.Exec(ctx, `UPDATE scheduled_jobs SET state='SCHEDULED',claim_token=NULL,lease_until=NULL,last_error='Lease expired; recovered' WHERE id IN (SELECT id FROM scheduled_jobs WHERE state='CLAIMED' AND lease_until<=$1 ORDER BY lease_until,id LIMIT 100 FOR UPDATE SKIP LOCKED)`, w.Clock.Now())
	if err == nil {
		observability.ObserveLeaseRecovery("runtime", "scheduled", tag.RowsAffected())
	}
	return err
}
func (w *Worker) claimReady(ctx context.Context, limit int) ([]Job, error) {
	if limit < 1 || limit > 100 {
		limit = 32
	}
	now := w.Clock.Now()
	// Keep the ordered jobs_due scan outside the one-to-one eligibility lookup.
	// Joining from a selective Active journey made LIMIT 32 sort/probe every
	// execution first. The guard is applied before LIMIT, so paused or locked
	// old jobs cannot starve eligible work behind them. Process still rechecks.
	rows, err := w.Pool.Query(ctx, `WITH due AS (
 SELECT j.id FROM scheduled_jobs j
 WHERE j.state='SCHEDULED' AND j.due_at<=$1
 AND (SELECT e.state IN ('RUNNING','WAITING') AND f.status IN ('Active','Sunset')
      FROM journey_executions e JOIN journeys f ON f.id=e.journey_id WHERE e.id=j.execution_id)
 ORDER BY j.due_at,j.id LIMIT $2 FOR UPDATE OF j SKIP LOCKED
 ), claimed AS (UPDATE scheduled_jobs j SET state='CLAIMED',claim_token=gen_random_uuid(),lease_until=$1+interval '30 seconds',attempts=j.attempts+1 FROM due WHERE j.id=due.id RETURNING j.*)
 SELECT c.id::text,c.workspace_id::text,c.execution_id::text,e.customer_id,e.journey_id::text,c.node_id,c.kind,c.claim_token::text,c.attempts,e.trace_parent,c.due_at FROM claimed c JOIN journey_executions e ON e.id=c.execution_id`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []Job{}
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &j.ExecutionID, &j.CustomerID, &j.JourneyID, &j.NodeID, &j.Kind, &j.Token, &j.Attempts, &j.TraceParent, &j.DueAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
func (w *Worker) Run(ctx context.Context) error {
	slots := make(chan struct{}, 8)
	completed := make(chan struct{}, 8)
	var group sync.WaitGroup
	defer group.Wait()
	var nextRecovery time.Time
	for ctx.Err() == nil {
		if !w.Clock.Now().Before(nextRecovery) {
			if err := w.recoverLeases(ctx); err != nil {
				if ctx.Err() == nil {
					w.Log.ErrorContext(ctx, "lease recovery failed")
				}
				if !sleep(ctx, time.Second) {
					break
				}
				continue
			}
			nextRecovery = w.Clock.Now().Add(time.Second)
		}
		available := cap(slots) - len(slots)
		if available == 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-completed:
				continue
			}
		}
		// Coalesce nearby completions into one bounded claim. Waiting for the
		// entire group would block behind a slow customer; claiming each freed
		// slot immediately instead multiplies database round trips under load.
		if available < cap(slots) {
			if !sleep(ctx, 2*time.Millisecond) {
				break
			}
			available = cap(slots) - len(slots)
		}
		jobs, err := w.claimReady(ctx, available)
		if err != nil {
			if ctx.Err() == nil {
				w.Log.ErrorContext(ctx, "job claim failed")
			}
			if !sleep(ctx, time.Second) {
				break
			}
			continue
		}
		for _, job := range jobs {
			if ctx.Err() != nil {
				break
			}
			slots <- struct{}{}
			group.Go(func() {
				defer func() {
					<-slots
					select {
					case completed <- struct{}{}:
					default:
					}
				}()
				if err := w.Process(ctx, job); err != nil && ctx.Err() == nil {
					w.retry(ctx, job)
				}
			})
		}
		if len(jobs) == 0 {
			timer := time.NewTimer(100 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-completed:
				timer.Stop()
			case <-timer.C:
			}
		}
	}
	return nil
}
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
func (w *Worker) retry(ctx context.Context, job Job) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	delay := time.Duration(1<<min(job.Attempts, 8)) * time.Second
	tag, err := w.Pool.Exec(ctx, `UPDATE scheduled_jobs SET state='SCHEDULED',due_at=$3,lease_until=NULL,claim_token=NULL,last_error='Step transaction failed; retry scheduled' WHERE id=$1 AND state='CLAIMED' AND claim_token=$2`, job.ID, job.Token, w.Clock.Now().Add(delay))
	if err == nil && tag.RowsAffected() > 0 {
		observability.ObserveRuntimeRetry()
	}
}
func (w *Worker) Process(ctx context.Context, job Job) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	ctx, span := observability.Start(observability.Parent(ctx, job.TraceParent), "runtime.step", attribute.String("execution.id", job.ExecutionID), attribute.String("node.id", job.NodeID))
	ctx, finish := observability.TrackOperation(ctx, "runtime_step")
	defer func() {
		finish(err)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.Log.ErrorContext(ctx, "step transaction failed", "job_id", job.ID, "error_class", observability.ErrorClass(err))
		}
		span.End()
	}()
	if !job.DueAt.IsZero() {
		observability.ObserveJobLag(job.Kind, w.Clock.Now().Sub(job.DueAt))
	}
	tx, err := w.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var profileJSON []byte
	if err := tx.QueryRow(ctx, `SELECT to_jsonb(c) FROM customers c WHERE workspace_id=$1 AND customer_id=$2 FOR UPDATE`, job.WorkspaceID, job.CustomerID).Scan(&profileJSON); err != nil {
		return err
	}
	var status string
	if err := tx.QueryRow(ctx, `SELECT status FROM journeys WHERE workspace_id=$1 AND id=$2 FOR SHARE`, job.WorkspaceID, job.JourneyID).Scan(&status); err != nil {
		return err
	}
	var state, current string
	var raw, stateJSON, eventJSON []byte
	err = tx.QueryRow(ctx, `SELECT e.state,e.current_node_id,v.workflow,e.context,jsonb_build_object('event_id',c.event_id,'event_name',c.event_name,'occurred_at',c.occurred_at,'properties',c.properties) FROM journey_executions e JOIN journey_versions v ON v.id=e.version_id JOIN customer_events c ON c.workspace_id=e.workspace_id AND c.event_id=COALESCE(e.context->>'_event_id',e.entry_event_id) WHERE e.workspace_id=$1 AND e.id=$2 FOR UPDATE OF e`, job.WorkspaceID, job.ExecutionID).Scan(&state, &current, &raw, &stateJSON, &eventJSON)
	if err != nil {
		return err
	}
	// Lock by identity before checking the fence. Otherwise the planner can
	// choose the small CLAIMED lease index and scan every in-flight job on
	// every step; the workspace predicate and all fence checks are retained.
	var claimed bool
	err = tx.QueryRow(ctx, `WITH locked AS MATERIALIZED (SELECT state,claim_token,lease_until FROM scheduled_jobs WHERE id=$1 AND workspace_id=$2 FOR UPDATE) SELECT true FROM locked WHERE state='CLAIMED' AND claim_token=$3 AND lease_until>$4`, job.ID, job.WorkspaceID, job.Token, w.Clock.Now()).Scan(&claimed)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	now := w.Clock.Now()
	terminal := state != "RUNNING" && state != "WAITING"
	if terminal || current != job.NodeID {
		_, err = tx.Exec(ctx, `UPDATE scheduled_jobs SET state='CANCELLED',lease_until=NULL WHERE id=$1 AND claim_token=$2`, job.ID, job.Token)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if status != "Active" && status != "Sunset" {
		_, err = tx.Exec(ctx, `UPDATE scheduled_jobs SET state='SCHEDULED',claim_token=NULL,lease_until=NULL WHERE id=$1 AND claim_token=$2`, job.ID, job.Token)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	var spec workflow.Workflow
	if err := json.Unmarshal(raw, &spec); err != nil {
		return err
	}
	if job.Attempts > 8 {
		return w.fail(ctx, tx, job, now, "Retry limit exhausted")
	}
	var profile, event, executionContext map[string]any
	_ = json.Unmarshal(profileJSON, &profile)
	_ = json.Unmarshal(eventJSON, &event)
	_ = json.Unmarshal(stateJSON, &executionContext)
	// The customer/journey/execution/job fences stay locked for this bounded
	// turn. Pure short nodes do not need another durable job or fresh profile
	// read; every node still writes its own transition history atomically.
	began := time.Now()
	for step := 0; step < maxTurnNodes; step++ {
		now = w.Clock.Now()
		node, ok := spec.Node(current)
		if !ok {
			return w.failAt(ctx, tx, job, current, now, "Node missing from immutable version")
		}
		resume := ""
		if step == 0 && job.Kind == "timer" {
			if node.Type == "wait_event" {
				resume = "timeout"
			} else {
				resume = "completed"
			}
		}
		decision, err := Decide(node, ContextData(profile, event, executionContext), now, resume)
		if err != nil {
			return w.failAt(ctx, tx, job, current, now, err.Error())
		}
		output, _ := json.Marshal(decision)
		stepState := "SUCCEEDED"
		var completedAt *time.Time = &now
		if decision.WaitUntil != nil {
			stepState = "WAITING"
			completedAt = nil
		}
		// RUNNING was previously inserted then overwritten inside this same
		// transaction, so readers never observed it. Write the final node state
		// once, preserving the original wait start and one entered metric.
		if _, err := tx.Exec(ctx, `INSERT INTO step_executions(workspace_id,execution_id,node_id,node_type,state,started_at,completed_at,output) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(execution_id,node_id) DO UPDATE SET node_type=excluded.node_type,state=excluded.state,completed_at=excluded.completed_at,output=excluded.output`, job.WorkspaceID, job.ExecutionID, node.ID, node.Type, stepState, now, completedAt, output); err != nil {
			return err
		}
		if decision.WaitUntil != nil {
			if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state='WAITING',updated_at=$2,current_node_id=$3 WHERE id=$1`, job.ExecutionID, now, node.ID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO scheduled_jobs(workspace_id,execution_id,node_id,kind,due_at) VALUES($1,$2,$3,'timer',$4) ON CONFLICT DO NOTHING`, job.WorkspaceID, job.ExecutionID, node.ID, *decision.WaitUntil); err != nil {
				return err
			}
			if decision.EventName != "" {
				if _, err := tx.Exec(ctx, `INSERT INTO event_waiters(workspace_id,execution_id,node_id,customer_id,event_name,deadline,created_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, job.WorkspaceID, job.ExecutionID, node.ID, job.CustomerID, decision.EventName, *decision.WaitUntil, now); err != nil {
					return err
				}
			}
		} else {
			if node.Type == "wait_event" {
				tag, err := tx.Exec(ctx, `UPDATE event_waiters SET state='TIMED_OUT' WHERE execution_id=$1 AND node_id=$2 AND state='WAITING'`, job.ExecutionID, node.ID)
				if err != nil {
					return err
				}
				if tag.RowsAffected() != 1 {
					return w.failAt(ctx, tx, job, current, now, "Event waiter already resolved")
				}
			}
			if decision.Message != nil {
				payload, _ := json.Marshal(decision.Message)
				if _, err := tx.Exec(ctx, `INSERT INTO action_outbox(workspace_id,execution_id,node_id,channel,payload,created_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, job.WorkspaceID, job.ExecutionID, node.ID, node.Type, payload, now); err != nil {
					return err
				}
			}
			if decision.Attribute != "" {
				attrs, _ := profile["attributes"].(map[string]any)
				if attrs == nil {
					attrs = map[string]any{}
				}
				next, err := ApplyAttribute(attrs, decision)
				if err != nil {
					return w.failAt(ctx, tx, job, current, now, err.Error())
				}
				attrs = next

				if _, err := tx.Exec(ctx, `UPDATE customers SET attributes=$3,updated_at=$4 WHERE workspace_id=$1 AND customer_id=$2`, job.WorkspaceID, job.CustomerID, attrs, now); err != nil {
					return err
				}
			}
			if decision.ExitReason != "" {
				state := "EXITED"
				if decision.ExitReason == "completed" {
					state = "COMPLETED"
				}
				if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state=$2,exit_reason=$3,completed_at=$4,updated_at=$4,current_node_id=$5 WHERE id=$1`, job.ExecutionID, state, decision.ExitReason, now, node.ID); err != nil {
					return err
				}
			} else {
				next := spec.Next(node.ID, decision.Port)
				if next == "" {
					return fmt.Errorf("workflow has no next node")
				}
				// Persist a boundary after externally visible intent/profile mutation,
				// or yield a durable continuation when this turn exhausts its budget.
				// A due timer acknowledges its deadline before dependent work is
				// queued. Otherwise rendering/conditions inside every timer turn
				// starve other due timers behind this one during a synchronized burst.
				if job.Kind == "timer" || decision.Message != nil || decision.Attribute != "" || step+1 >= maxTurnNodes || time.Since(began) >= maxTurnDuration {
					if err := scheduleNext(ctx, tx, job.WorkspaceID, job.ExecutionID, next, now); err != nil {
						return err
					}
				} else {
					current = next
					continue
				}
			}
		}
		break
	}

	if _, err := tx.Exec(ctx, `UPDATE scheduled_jobs SET state='SUCCEEDED',lease_until=NULL WHERE id=$1 AND claim_token=$2`, job.ID, job.Token); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func scheduleNext(ctx context.Context, tx pgx.Tx, workspace, execution, next string, now time.Time) error {
	if next == "" {
		return fmt.Errorf("workflow has no next node")
	}
	if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state='RUNNING',current_node_id=$2,updated_at=$3 WHERE id=$1`, execution, next, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO scheduled_jobs(workspace_id,execution_id,node_id,due_at) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, workspace, execution, next, now)
	return err
}

// An inline node has no separate queued job. Materialize its failure so
// manual recovery starts at the failed node, without replaying committed
// preceding mutations or losing the original claim's completion.
func (w *Worker) failAt(ctx context.Context, tx pgx.Tx, job Job, node string, now time.Time, message string) error {
	if node == job.NodeID {
		return w.fail(ctx, tx, job, now, message)
	}
	if _, err := tx.Exec(ctx, `UPDATE scheduled_jobs SET state='SUCCEEDED',lease_until=NULL WHERE id=$1 AND claim_token=$2`, job.ID, job.Token); err != nil {
		return err
	}
	failed := job
	failed.NodeID = node
	if err := tx.QueryRow(ctx, `INSERT INTO scheduled_jobs(workspace_id,execution_id,node_id,due_at,state,attempts,last_error) VALUES($1,$2,$3,$4,'FAILED',1,$5) RETURNING id::text`, job.WorkspaceID, job.ExecutionID, node, now, message).Scan(&failed.ID); err != nil {
		return err
	}
	return w.fail(ctx, tx, failed, now, message)
}
func (w *Worker) fail(ctx context.Context, tx pgx.Tx, job Job, now time.Time, message string) error {
	observability.SetOutcome(ctx, "failed")
	if _, err := tx.Exec(ctx, `INSERT INTO step_executions(workspace_id,execution_id,node_id,node_type,state,started_at,completed_at,output) VALUES($1,$2,$3,'unknown','FAILED',$4,$4,jsonb_build_object('error',$5::text)) ON CONFLICT(execution_id,node_id) DO UPDATE SET state='FAILED',completed_at=$4,output=jsonb_build_object('error',$5::text)`, job.WorkspaceID, job.ExecutionID, job.NodeID, now, message); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state='FAILED',exit_reason='step_failed',updated_at=$2,completed_at=$2,current_node_id=$3 WHERE id=$1`, job.ExecutionID, now, job.NodeID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE scheduled_jobs SET state=CASE WHEN id=$2 THEN 'FAILED' ELSE 'CANCELLED' END,lease_until=NULL,last_error=$3 WHERE execution_id=$1 AND state IN ('SCHEDULED','CLAIMED')`, job.ExecutionID, job.ID, message); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE event_waiters SET state='CANCELLED' WHERE execution_id=$1 AND state='WAITING'`, job.ExecutionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE action_outbox SET state='CANCELLED' WHERE execution_id=$1 AND state='PENDING'`, job.ExecutionID); err != nil {
		return err
	}
	if err := cancelDeliveries(ctx, tx, job.ExecutionID, "step_failed", now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
