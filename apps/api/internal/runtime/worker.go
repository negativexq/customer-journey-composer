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
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
)

type Worker struct {
	Pool  *pgxpool.Pool
	Clock Clock
	Log   *slog.Logger
}
type Job struct {
	ID, WorkspaceID, ExecutionID, CustomerID, JourneyID, NodeID, Kind, Token string
	Attempts                                                                 int
}

func NewWorker(pool *pgxpool.Pool, clock Clock) *Worker {
	if clock == nil {
		clock = RealClock{}
	}
	return &Worker{Pool: pool, Clock: clock, Log: slog.Default()}
}
func (w *Worker) Claim(ctx context.Context, limit int) ([]Job, error) {
	if limit < 1 || limit > 100 {
		limit = 32
	}
	now := w.Clock.Now()
	// Bounded stale-lease recovery; the fencing token is replaced on every claim.
	_, err := w.Pool.Exec(ctx, `UPDATE scheduled_jobs SET state='SCHEDULED',claim_token=NULL,lease_until=NULL,last_error='Lease expired; recovered' WHERE id IN (SELECT id FROM scheduled_jobs WHERE state='CLAIMED' AND lease_until<=$1 ORDER BY lease_until,id LIMIT 100 FOR UPDATE SKIP LOCKED)`, now)
	if err != nil {
		return nil, err
	}
	rows, err := w.Pool.Query(ctx, `WITH due AS (
 SELECT j.id FROM scheduled_jobs j JOIN journey_executions e ON e.id=j.execution_id JOIN journeys f ON f.id=e.journey_id
 WHERE j.state='SCHEDULED' AND j.due_at<=$1 AND e.state IN ('RUNNING','WAITING') AND f.status IN ('Active','Sunset')
 ORDER BY j.due_at,j.id LIMIT $2 FOR UPDATE OF j SKIP LOCKED
 ), claimed AS (UPDATE scheduled_jobs j SET state='CLAIMED',claim_token=gen_random_uuid(),lease_until=$1+interval '30 seconds',attempts=j.attempts+1 FROM due WHERE j.id=due.id RETURNING j.*)
 SELECT c.id::text,c.workspace_id::text,c.execution_id::text,e.customer_id,e.journey_id::text,c.node_id,c.kind,c.claim_token::text,c.attempts FROM claimed c JOIN journey_executions e ON e.id=c.execution_id`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	jobs := []Job{}
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.WorkspaceID, &j.ExecutionID, &j.CustomerID, &j.JourneyID, &j.NodeID, &j.Kind, &j.Token, &j.Attempts); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}
func (w *Worker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		jobs, err := w.Claim(ctx, 32)
		if err != nil {
			w.Log.Error("job claim failed")
			if !sleep(ctx, time.Second) {
				break
			}
			continue
		}
		if len(jobs) == 0 {
			if !sleep(ctx, 100*time.Millisecond) {
				break
			}
			continue
		}
		sem := make(chan struct{}, 8)
		var group sync.WaitGroup
		for _, job := range jobs {
			if ctx.Err() != nil {
				break
			}
			sem <- struct{}{}
			group.Go(func() {
				defer func() { <-sem }()
				if err := w.Process(ctx, job); err != nil && ctx.Err() == nil {
					w.Log.Error("step transaction failed", "job_id", job.ID)
					w.retry(ctx, job)
				}
			})
		}
		group.Wait()
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
	delay := time.Duration(1<<min(job.Attempts, 8)) * time.Second
	_, _ = w.Pool.Exec(ctx, `UPDATE scheduled_jobs SET state='SCHEDULED',due_at=$3,lease_until=NULL,claim_token=NULL,last_error='Step transaction failed; retry scheduled' WHERE id=$1 AND state='CLAIMED' AND claim_token=$2`, job.ID, job.Token, w.Clock.Now().Add(delay))
}
func (w *Worker) Process(ctx context.Context, job Job) error {
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
	var claimed bool
	err = tx.QueryRow(ctx, `SELECT true FROM scheduled_jobs WHERE id=$1 AND workspace_id=$2 AND state='CLAIMED' AND claim_token=$3 AND lease_until>$4 FOR UPDATE`, job.ID, job.WorkspaceID, job.Token, w.Clock.Now()).Scan(&claimed)
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
	node, ok := spec.Node(current)
	if !ok {
		return w.fail(ctx, tx, job, now, "Node missing from immutable version")
	}
	if job.Attempts > 8 {
		return w.fail(ctx, tx, job, now, "Retry limit exhausted")
	}
	var profile, event, executionContext map[string]any
	_ = json.Unmarshal(profileJSON, &profile)
	_ = json.Unmarshal(eventJSON, &event)
	_ = json.Unmarshal(stateJSON, &executionContext)
	resume := ""
	if job.Kind == "timer" {
		if node.Type == "wait_event" {
			resume = "timeout"
		} else {
			resume = "completed"
		}
	}
	decision, err := Decide(node, ContextData(profile, event, executionContext), now, resume)
	if err != nil {
		return w.fail(ctx, tx, job, now, err.Error())
	}
	if _, err := tx.Exec(ctx, `INSERT INTO step_executions(workspace_id,execution_id,node_id,node_type,state,started_at) VALUES($1,$2,$3,$4,'RUNNING',$5) ON CONFLICT(execution_id,node_id) DO NOTHING`, job.WorkspaceID, job.ExecutionID, node.ID, node.Type, now); err != nil {
		return err
	}
	output, _ := json.Marshal(decision)
	if decision.WaitUntil != nil {
		if _, err := tx.Exec(ctx, `UPDATE step_executions SET state='WAITING',output=$3 WHERE execution_id=$1 AND node_id=$2`, job.ExecutionID, node.ID, output); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state='WAITING',updated_at=$2 WHERE id=$1`, job.ExecutionID, now); err != nil {
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
				return w.fail(ctx, tx, job, now, "Event waiter already resolved")
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
			switch decision.Operation {
			case "set":
				attrs[decision.Attribute] = decision.Value
			case "unset":
				delete(attrs, decision.Attribute)
			case "increment":
				by, ok := decision.Value.(float64)
				if !ok {
					return w.fail(ctx, tx, job, now, "Increment requires a number")
				}
				previous := 0.0
				if value, exists := attrs[decision.Attribute]; exists {
					var ok bool
					previous, ok = value.(float64)
					if !ok {
						return w.fail(ctx, tx, job, now, "Existing attribute is not numeric")
					}
				}
				attrs[decision.Attribute] = previous + by
			default:
				return w.fail(ctx, tx, job, now, "Invalid attribute operation")
			}
			if _, err := tx.Exec(ctx, `UPDATE customers SET attributes=$3,updated_at=$4 WHERE workspace_id=$1 AND customer_id=$2`, job.WorkspaceID, job.CustomerID, attrs, now); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE step_executions SET state='SUCCEEDED',completed_at=$3,output=$4 WHERE execution_id=$1 AND node_id=$2`, job.ExecutionID, node.ID, now, output); err != nil {
			return err
		}
		if decision.ExitReason != "" {
			state := "EXITED"
			if decision.ExitReason == "completed" {
				state = "COMPLETED"
			}
			if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state=$2,exit_reason=$3,completed_at=$4,updated_at=$4 WHERE id=$1`, job.ExecutionID, state, decision.ExitReason, now); err != nil {
				return err
			}
		} else {
			if err := scheduleNext(ctx, tx, job.WorkspaceID, job.ExecutionID, spec.Next(node.ID, decision.Port), now); err != nil {
				return err
			}
		}
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
func (w *Worker) fail(ctx context.Context, tx pgx.Tx, job Job, now time.Time, message string) error {
	if _, err := tx.Exec(ctx, `INSERT INTO step_executions(workspace_id,execution_id,node_id,node_type,state,started_at,completed_at,output) VALUES($1,$2,$3,'unknown','FAILED',$4,$4,jsonb_build_object('error',$5::text)) ON CONFLICT(execution_id,node_id) DO UPDATE SET state='FAILED',completed_at=$4,output=jsonb_build_object('error',$5::text)`, job.WorkspaceID, job.ExecutionID, job.NodeID, now, message); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state='FAILED',exit_reason='step_failed',updated_at=$2,completed_at=$2 WHERE id=$1`, job.ExecutionID, now); err != nil {
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
