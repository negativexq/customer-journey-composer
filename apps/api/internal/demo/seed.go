package demo

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/catalog"
	sim "github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"time"
)

// seed inserts fictional design/test data once. It creates no live event,
// enrollment, delivery, conversion or fabricated performance measurement.
func seed(ctx context.Context, tx pgx.Tx, workspace, user string) error {
	var seeded bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM demo_seed_state WHERE workspace_id=$1 AND version>=1)`, workspace).Scan(&seeded); err != nil {
		return err
	}
	if seeded {
		return nil
	}
	profiles := []struct {
		ID, Name string
		Orders   int
		Inactive bool
	}{{"demo_new", "Ada", 0, false}, {"demo_returning", "Deniz", 3, false}, {"demo_inactive", "Ekin", 0, true}}
	for i, p := range profiles {
		attrs := map[string]any{"first_name": p.Name, "total_orders": p.Orders, "purchased": false, "activated": false, "inactive": p.Inactive, "cart_amount": 2450}
		if _, err := tx.Exec(ctx, `INSERT INTO customers(workspace_id,customer_id,email,phone,locale,timezone,attributes,consents) VALUES($1,$2,$3,$4,'en','Europe/Istanbul',$5,'{"email":true,"sms":true}') ON CONFLICT(workspace_id,customer_id) DO NOTHING`, workspace, p.ID, p.ID+"@example.invalid", []string{"+905550000001", "+905550000002", "+905550000003"}[i], attrs); err != nil {
			return err
		}
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, template := range catalog.Builtins() {
		flow, _ := json.Marshal(template.Workflow)
		var journey string
		if err := tx.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,description,tags,draft) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, workspace, template.Name, template.Description, template.Tags, flow).Scan(&journey); err != nil {
			return err
		}
		hash := sim.Hash(template.Workflow)
		if _, err := tx.Exec(ctx, `INSERT INTO sandbox_workflows(workspace_id,workflow_hash,workflow) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, workspace, hash, flow); err != nil {
			return err
		}
		var customer []byte
		if err := tx.QueryRow(ctx, `SELECT to_jsonb(c)-'workspace_id' FROM customers c WHERE workspace_id=$1 AND customer_id='demo_new'`, workspace).Scan(&customer); err != nil {
			return err
		}
		var copied map[string]any
		if err := json.Unmarshal(customer, &copied); err != nil {
			return err
		}
		normal := sim.Scenario{Name: template.Name + " — full path", StartTime: now, CustomerID: "demo_new", Customer: copied, Entry: sim.Event{EventID: "demo_entry", EventName: template.Workflow.Start().Config.EventName, Properties: map[string]any{"cart_id": "demo_cart", "amount": 2450, "currency": "TRY"}}, Assertions: []sim.Assertion{{Type: "state", Expected: "COMPLETED"}, {Type: "message_sent", Channel: "email"}, {Type: "message_sent", Channel: "sms"}, {Type: "no_duplicate_delivery", Expected: true}}}
		scenarios := []sim.Scenario{normal}
		if template.ID == "abandoned-cart" {
			early := normal
			early.Name = "Abandoned cart — purchase before reminder"
			early.Events = []sim.TimedEvent{{AfterSeconds: 3600, Event: sim.Event{EventID: "demo_purchase", EventName: "purchase_completed"}}}
			early.Assertions = []sim.Assertion{{Type: "state", Expected: "EXITED"}, {Type: "exit_reason", Expected: "converted"}, {Type: "no_message"}}
			scenarios = append(scenarios, early)
		}
		for _, scenario := range scenarios {
			_, _, normalized, err := sim.New(template.Workflow, scenario)
			if err != nil {
				return err
			}
			raw, _ := json.Marshal(normalized)
			if _, err = tx.Exec(ctx, `INSERT INTO test_scenarios(workspace_id,journey_id,draft_revision,workflow_hash,source_customer_id,name,scenario,created_by) VALUES($1,$2,1,$3,'demo_new',$4,$5,$6)`, workspace, journey, hash, normalized.Name, raw, user); err != nil {
				return err
			}
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO demo_seed_state(workspace_id,version) VALUES($1,1)`, workspace)
	return err
}
