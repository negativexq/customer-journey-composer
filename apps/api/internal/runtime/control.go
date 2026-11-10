package runtime

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"time"
)

type ControlConflict struct{ Message string }

func (e *ControlConflict) Error() string { return e.Message }

// Control executes within the caller's transaction so the audit and transition
// commit together. Lock order matches ingestion: customer, journey, execution.
func Control(ctx context.Context, tx pgx.Tx, workspace, id, action string, now time.Time) (map[string]any, error) {
	var customer, journey string
	if err := tx.QueryRow(ctx, `SELECT customer_id,journey_id::text FROM journey_executions WHERE workspace_id=$1 AND id=$2`, workspace, id).Scan(&customer, &journey); err != nil {
		return nil, err
	}
	var locked bool
	if err := tx.QueryRow(ctx, `SELECT true FROM customers WHERE workspace_id=$1 AND customer_id=$2 FOR UPDATE`, workspace, customer).Scan(&locked); err != nil {
		return nil, err
	}
	var lifecycle string
	if err := tx.QueryRow(ctx, `SELECT status FROM journeys WHERE workspace_id=$1 AND id=$2 FOR SHARE`, workspace, journey).Scan(&lifecycle); err != nil {
		return nil, err
	}
	var state, node string
	if err := tx.QueryRow(ctx, `SELECT state,current_node_id FROM journey_executions WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, workspace, id).Scan(&state, &node); err != nil {
		return nil, err
	}
	details := map[string]any{"from": state, "node_id": node}
	if action == "cancel" {
		if state == "CANCELLED" {
			details["unchanged"] = true
			return details, nil
		}
		if state != "RUNNING" && state != "WAITING" {
			return nil, &ControlConflict{"Only active executions can be cancelled"}
		}
		if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state='CANCELLED',exit_reason='manually_cancelled',completed_at=$3,updated_at=$3 WHERE workspace_id=$1 AND id=$2`, workspace, id, now); err != nil {
			return nil, err
		}
		for _, sql := range []string{
			`UPDATE scheduled_jobs SET state='CANCELLED',claim_token=NULL,lease_until=NULL WHERE execution_id=$1 AND state IN ('SCHEDULED','CLAIMED')`,
			`UPDATE event_waiters SET state='CANCELLED' WHERE execution_id=$1 AND state='WAITING'`,
			`UPDATE action_outbox SET state='CANCELLED' WHERE execution_id=$1 AND state='PENDING'`,
		} {
			if _, err := tx.Exec(ctx, sql, id); err != nil {
				return nil, err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE step_executions SET state='SKIPPED',completed_at=$2,output=output||'{"reason":"manually_cancelled"}' WHERE execution_id=$1 AND state IN ('SCHEDULED','RUNNING','WAITING')`, id, now); err != nil {
			return nil, err
		}
		if err := cancelDeliveries(ctx, tx, id, "manually_cancelled", now); err != nil {
			return nil, err
		}
		details["to"] = "CANCELLED"
		return details, nil
	}
	if action != "retry" {
		return nil, fmt.Errorf("unknown execution control")
	}
	if state != "FAILED" {
		return nil, &ControlConflict{"Only failed executions can be recovered"}
	}
	if lifecycle != "Active" && lifecycle != "Sunset" && lifecycle != "Paused" {
		return nil, &ControlConflict{"Journey must be active, sunset or paused before recovery"}
	}
	var active, actionExists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM journey_executions WHERE workspace_id=$1 AND journey_id=$2 AND customer_id=$3 AND state IN ('RUNNING','WAITING')),EXISTS(SELECT 1 FROM action_outbox WHERE execution_id=$4 AND node_id=$5)`, workspace, journey, customer, id, node).Scan(&active, &actionExists); err != nil {
		return nil, err
	}
	if active || actionExists {
		return nil, &ControlConflict{"Another execution or a logical action already exists; recovery cannot duplicate it"}
	}
	var job string
	var attempts, retries int
	if err := tx.QueryRow(ctx, `SELECT id::text,attempts,manual_retries FROM scheduled_jobs WHERE execution_id=$1 AND node_id=$2 AND state='FAILED' ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, id, node).Scan(&job, &attempts, &retries); err != nil {
		if err == pgx.ErrNoRows {
			return nil, &ControlConflict{"No failed job is available for recovery"}
		}
		return nil, err
	}
	if retries >= 3 {
		return nil, &ControlConflict{"Manual recovery limit reached"}
	}
	if _, err := tx.Exec(ctx, `UPDATE journey_executions SET state='RUNNING',exit_reason=NULL,completed_at=NULL,updated_at=$2 WHERE id=$1`, id, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `UPDATE scheduled_jobs SET state='SCHEDULED',due_at=$2,attempts=0,manual_retries=manual_retries+1,claim_token=NULL,lease_until=NULL,last_error='Manually recovered after failure' WHERE id=$1`, job, now); err != nil {
		return nil, err
	}
	details["to"] = "RUNNING"
	details["previous_attempts"] = attempts
	details["manual_recovery"] = retries + 1
	return details, nil
}
