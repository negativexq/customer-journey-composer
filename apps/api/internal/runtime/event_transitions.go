package runtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

// applyEvent runs while the customer row is locked. Event and timeout updates
// share that same lock and execution row; a cancelled claim cannot later win.
func applyEvent(ctx context.Context, tx pgx.Tx, workspace string, event Event, data map[string]any, now time.Time) error {
	rows, err := tx.Query(ctx, `SELECT e.id::text,v.workflow FROM journey_executions e JOIN journey_versions v ON v.id=e.version_id JOIN customer_events entry_event ON entry_event.workspace_id=e.workspace_id AND entry_event.event_id=e.entry_event_id WHERE e.workspace_id=$1 AND e.customer_id=$2 AND e.exit_event_name=$3 AND entry_event.occurred_at<=$4 AND e.state IN ('RUNNING','WAITING') ORDER BY e.id FOR UPDATE OF e`, workspace, event.CustomerID, event.EventName, event.OccurredAt)
	if err != nil {
		return err
	}
	type exit struct {
		id   string
		spec workflow.Workflow
	}
	exits := []exit{}
	for rows.Next() {
		var x exit
		var b []byte
		if err := rows.Scan(&x.id, &b); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal(b, &x.spec); err != nil {
			rows.Close()
			return err
		}
		exits = append(exits, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, x := range exits {
		if x.spec.GlobalExit == nil || (x.spec.GlobalExit.Filters != nil && !workflow.Evaluate(*x.spec.GlobalExit.Filters, data)) {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state='EXITED',exit_reason='converted',completed_at=$2,updated_at=$2,context=context||jsonb_build_object('exit_event_id',$3::text) WHERE id=$1`, x.id, now, event.EventID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE scheduled_jobs SET state='CANCELLED',lease_until=NULL,claim_token=NULL WHERE execution_id=$1 AND state IN ('SCHEDULED','CLAIMED')`, x.id); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE step_executions SET state='SKIPPED',completed_at=$2,output=output||'{"reason":"global_exit"}' WHERE execution_id=$1 AND state IN ('SCHEDULED','RUNNING','WAITING')`, x.id, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE event_waiters SET state='CANCELLED' WHERE execution_id=$1 AND state='WAITING'`, x.id); err != nil {
			return err
		}
		if err := cancelDeliveries(ctx, tx, x.id, "global_exit", now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE action_outbox SET state='CANCELLED' WHERE execution_id=$1 AND state='PENDING'`, x.id); err != nil {
			return err
		}
	}
	rows, err = tx.Query(ctx, `SELECT w.execution_id::text,w.node_id,v.workflow FROM event_waiters w JOIN journey_executions e ON e.id=w.execution_id JOIN journey_versions v ON v.id=e.version_id WHERE w.workspace_id=$1 AND w.customer_id=$2 AND w.event_name=$3 AND w.state='WAITING' AND e.state='WAITING' AND e.current_node_id=w.node_id AND w.deadline>=$4 AND w.created_at<=$5 ORDER BY w.execution_id FOR UPDATE OF w,e`, workspace, event.CustomerID, event.EventName, now, event.OccurredAt)
	if err != nil {
		return err
	}
	type waiter struct {
		execution, node string
		spec            workflow.Workflow
	}
	waiters := []waiter{}
	for rows.Next() {
		var x waiter
		var b []byte
		if err := rows.Scan(&x.execution, &x.node, &b); err != nil {
			rows.Close()
			return err
		}
		if err := json.Unmarshal(b, &x.spec); err != nil {
			rows.Close()
			return err
		}
		waiters = append(waiters, x)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, x := range waiters {
		node, ok := x.spec.Node(x.node)
		if !ok {
			continue
		}
		if node.Config.Filters != nil && !workflow.Evaluate(*node.Config.Filters, data) {
			continue
		}
		tag, err := tx.Exec(ctx, `UPDATE event_waiters SET state='MATCHED',matched_event_id=$3 WHERE execution_id=$1 AND node_id=$2 AND state='WAITING'`, x.execution, x.node, event.EventID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			continue
		}
		if _, err := tx.Exec(ctx, `UPDATE scheduled_jobs SET state='CANCELLED',claim_token=NULL,lease_until=NULL WHERE execution_id=$1 AND node_id=$2 AND kind='timer' AND state IN ('SCHEDULED','CLAIMED')`, x.execution, x.node); err != nil {
			return err
		}
		decision, err := Decide(node, data, now, "matched")
		if err != nil {
			return err
		}
		output, _ := json.Marshal(decision)
		if _, err := tx.Exec(ctx, `UPDATE step_executions SET state='SUCCEEDED',completed_at=$3,output=$4 WHERE execution_id=$1 AND node_id=$2`, x.execution, x.node, now, output); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE journey_executions SET context=context||jsonb_build_object('_event_id',$2::text) WHERE id=$1`, x.execution, event.EventID); err != nil {
			return err
		}
		if err := scheduleNext(ctx, tx, workspace, x.execution, x.spec.Next(x.node, decision.Port), now); err != nil {
			return err
		}
	}
	return nil
}
