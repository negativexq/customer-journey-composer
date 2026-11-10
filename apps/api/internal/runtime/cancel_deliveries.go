package runtime

import (
	"context"
	"github.com/jackc/pgx/v5"
	"time"
)

func cancelDeliveries(ctx context.Context, tx pgx.Tx, execution, reason string, now time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE delivery_jobs SET status='CANCELLED',claim_token=NULL,lease_until=NULL,decision=decision||jsonb_build_object('eligible',false,'reason',$2::text) WHERE execution_id=$1 AND status IN ('SCHEDULED','CLAIMED')`, execution, reason); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE delivery_attempts a SET status='CANCELLED',completed_at=$2,response=$3 FROM delivery_jobs d WHERE a.delivery_id=d.id AND d.execution_id=$1 AND d.status='CANCELLED' AND a.status='STARTED'`, execution, now, reason); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE delivery_jobs SET cancel_requested=true WHERE execution_id=$1 AND status='DISPATCHING'`, execution)
	return err
}
