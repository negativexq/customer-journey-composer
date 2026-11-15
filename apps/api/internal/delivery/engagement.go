package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
)

// EngagementKinds are the reports a delivery can receive after acceptance.
var EngagementKinds = []string{"delivered", "bounced", "opened", "clicked", "unsubscribed"}

// EngagementEvent is the customer event a journey sees for a report.
func EngagementEvent(kind string) string { return "message_" + kind }

// RecordEngagement keeps the first report of a kind for a delivery. For a
// journey message it then hands the report to the runtime as a customer event,
// so a step waiting for an open or a click can move on. Repeating it is safe:
// the report is kept once, and the event ID is derived from the delivery and
// kind, with the stored time and detail, so the runtime sees it once.
func RecordEngagement(ctx context.Context, pool *pgxpool.Pool, workspace, deliveryID, kind, source string, detail map[string]any, at, now time.Time) (bool, error) {
	if detail == nil {
		detail = map[string]any{}
	}
	tag, err := pool.Exec(ctx, `INSERT INTO delivery_events(workspace_id,delivery_id,kind,occurred_at,source,detail) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, workspace, deliveryID, kind, at.UTC(), source, detail)
	if err != nil {
		return false, err
	}
	first := tag.RowsAffected() == 1
	var mode, customer, execution, node, channel, journey string
	var occurred time.Time
	var stored map[string]any
	var ingested *time.Time
	err = pool.QueryRow(ctx, `SELECT d.mode,COALESCE(d.customer_id,''),COALESCE(d.execution_id::text,''),COALESCE(d.node_id,''),d.channel,COALESCE(x.journey_id::text,''),e.occurred_at,e.detail,e.ingested_at FROM delivery_events e JOIN delivery_jobs d ON d.id=e.delivery_id LEFT JOIN journey_executions x ON x.id=d.execution_id WHERE e.workspace_id=$1 AND e.delivery_id=$2 AND e.kind=$3`, workspace, deliveryID, kind).Scan(&mode, &customer, &execution, &node, &channel, &journey, &occurred, &stored, &ingested)
	if err != nil {
		return first, err
	}
	// Test sends have no customer journey to move.
	if mode != "LIVE" || customer == "" || execution == "" || ingested != nil {
		return first, nil
	}
	properties := map[string]any{"delivery_id": deliveryID, "journey_id": journey, "execution_id": execution, "node_id": node, "channel": channel}
	if url, ok := stored["url"].(string); ok && url != "" {
		properties["url"] = url
	}
	event := engine.Event{EventID: "delivery:" + deliveryID + ":" + kind, CustomerID: customer, EventName: EngagementEvent(kind), OccurredAt: occurred, Properties: properties}
	if _, err := engine.Ingest(ctx, pool, workspace, event, now); err != nil {
		var input *engine.InputError
		// A customer removed since the send has no journey left to move.
		if !errors.As(err, &input) {
			return first, err
		}
	}
	_, err = pool.Exec(ctx, `UPDATE delivery_events SET ingested_at=$4 WHERE workspace_id=$1 AND delivery_id=$2 AND kind=$3 AND ingested_at IS NULL`, workspace, deliveryID, kind, now)
	return first, err
}

// codesJSON is the parameter the delivery inserts read their codes from.
func codesJSON(codes []TrackingCode) []byte {
	if codes == nil {
		codes = []TrackingCode{}
	}
	raw, _ := json.Marshal(codes)
	return raw
}

// simulatedReport is what the mock providers report a moment after they
// accept a message. Nothing is delivered for real; the report exists so the
// demo can show and branch on it. Recipients set aside for it bounce.
func simulatedReport(channel, to string) string {
	if channel == "sms" && strings.HasSuffix(to, "0000") {
		return "bounced"
	}
	if channel == "email" && (strings.HasPrefix(to, "bounce@") || strings.Contains(to, "+bounce@")) {
		return "bounced"
	}
	return "delivered"
}

// Unsubscribe withdraws the email consent of the customer a message went to,
// then records the report. A test message changes no customer. Consent is
// read again before every send, so emails already queued end as blocked.
func Unsubscribe(ctx context.Context, pool *pgxpool.Pool, workspace, deliveryID string, now time.Time) error {
	if _, err := pool.Exec(ctx, `UPDATE customers c SET consents=c.consents||'{"email":false}'::jsonb,updated_at=$3 FROM delivery_jobs d WHERE d.workspace_id=$1 AND d.id=$2 AND d.mode='LIVE' AND c.workspace_id=d.workspace_id AND c.customer_id=d.customer_id AND (c.consents->>'email')::boolean IS DISTINCT FROM false`, workspace, deliveryID, now); err != nil {
		return err
	}
	_, err := RecordEngagement(ctx, pool, workspace, deliveryID, "unsubscribed", "tracking", nil, now, now)
	return err
}
