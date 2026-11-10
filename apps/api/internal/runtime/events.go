package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ofk/customer-journey-composer/apps/api/internal/workflow"
)

type Event struct {
	EventID    string         `json:"event_id" minLength:"1" maxLength:"128"`
	CustomerID string         `json:"customer_id" minLength:"1" maxLength:"128"`
	EventName  string         `json:"event_name" pattern:"^[a-z][a-z0-9_]{0,79}$"`
	OccurredAt time.Time      `json:"occurred_at"`
	Properties map[string]any `json:"properties,omitempty"`
}
type Ingestion struct {
	EventID    string   `json:"event_id"`
	Status     string   `json:"status"`
	Executions []string `json:"executions"`
	Error      string   `json:"error,omitempty"`
}
type InputError struct {
	Message  string
	Conflict bool
}

func (e *InputError) Error() string { return e.Message }

var eventPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,79}$`)

func ValidateEvent(event Event, now time.Time) error {
	if len(event.EventID) < 1 || len(event.EventID) > 128 || len(event.CustomerID) < 1 || len(event.CustomerID) > 128 || !eventPattern.MatchString(event.EventName) || event.OccurredAt.IsZero() {
		return &InputError{Message: "Event ID, customer ID, valid event name and occurred_at are required"}
	}
	if event.OccurredAt.After(now.Add(5 * time.Minute)) {
		return &InputError{Message: "Events more than five minutes in the future are rejected"}
	}
	data, _ := json.Marshal(event.Properties)
	if len(data) > 65536 {
		return &InputError{Message: "Event properties exceed 64 KiB"}
	}
	return nil
}
func Ingest(ctx context.Context, pool *pgxpool.Pool, workspace string, event Event, now time.Time) (Ingestion, error) {
	result := Ingestion{EventID: event.EventID, Status: "ACCEPTED", Executions: []string{}}
	if err := ValidateEvent(event, now); err != nil {
		return result, err
	}
	if event.Properties == nil {
		event.Properties = map[string]any{}
	}
	event.OccurredAt = event.OccurredAt.UTC()
	canonical, _ := json.Marshal(event)
	hash := sha256.Sum256(canonical)
	fingerprint := hex.EncodeToString(hash[:])
	tx, err := pool.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var profileJSON []byte
	// All event/runtime transitions for a customer acquire this lock first. This
	// serializes enrollment, exit, timers and workspace frequency-cap decisions.
	err = tx.QueryRow(ctx, `SELECT to_jsonb(c) FROM customers c WHERE workspace_id=$1 AND customer_id=$2 FOR UPDATE`, workspace, event.CustomerID).Scan(&profileJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, &InputError{Message: "Customer does not exist in this workspace"}
	}
	if err != nil {
		return result, err
	}
	if event.OccurredAt.Before(now.Add(-24 * time.Hour)) {
		result.Status = "LATE"
	}
	tag, err := tx.Exec(ctx, `INSERT INTO customer_events(workspace_id,event_id,customer_id,event_name,occurred_at,properties,payload_hash,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (workspace_id,event_id) DO NOTHING`, workspace, event.EventID, event.CustomerID, event.EventName, event.OccurredAt, event.Properties, fingerprint, result.Status)
	if err != nil {
		return result, err
	}
	if tag.RowsAffected() == 0 {
		var existing string
		err := tx.QueryRow(ctx, `SELECT payload_hash FROM customer_events WHERE workspace_id=$1 AND event_id=$2`, workspace, event.EventID).Scan(&existing)
		if err != nil {
			return result, err
		}
		if existing != fingerprint {
			return result, &InputError{Message: "Event ID was already used with a different payload", Conflict: true}
		}
		if _, err := tx.Exec(ctx, `UPDATE customer_events SET duplicate_count=duplicate_count+1 WHERE workspace_id=$1 AND event_id=$2`, workspace, event.EventID); err != nil {
			return result, err
		}
		result.Status = "DUPLICATE"
		return result, tx.Commit(ctx)
	}
	if result.Status == "LATE" {
		return result, tx.Commit(ctx)
	}
	var profile map[string]any
	_ = json.Unmarshal(profileJSON, &profile)
	data := ContextData(profile, map[string]any{"event_id": event.EventID, "event_name": event.EventName, "occurred_at": event.OccurredAt.Format(time.RFC3339Nano), "properties": event.Properties}, map[string]any{})
	if err := applyEvent(ctx, tx, workspace, event, data, now); err != nil {
		return result, err
	}
	rows, err := tx.Query(ctx, `SELECT j.id::text,j.current_version_id::text,v.workflow FROM journeys j JOIN journey_versions v ON v.id=j.current_version_id WHERE j.workspace_id=$1 AND j.status='Active' AND j.entry_event_name=$2 ORDER BY j.id FOR SHARE OF j`, workspace, event.EventName)
	if err != nil {
		return result, err
	}
	type match struct {
		journey, version string
		spec             workflow.Workflow
	}
	matches := []match{}
	for rows.Next() {
		var m match
		var raw []byte
		if err := rows.Scan(&m.journey, &m.version, &raw); err != nil {
			rows.Close()
			return result, err
		}
		if err := json.Unmarshal(raw, &m.spec); err != nil {
			rows.Close()
			return result, err
		}
		matches = append(matches, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	for _, m := range matches {
		start := m.spec.Start()
		if start.Config.Filters != nil && !workflow.Evaluate(*start.Config.Filters, data) {
			continue
		}
		var active bool
		var previous bool
		var last *time.Time
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM journey_executions WHERE workspace_id=$1 AND journey_id=$2 AND customer_id=$3 AND state IN ('RUNNING','WAITING')),EXISTS(SELECT 1 FROM journey_executions WHERE workspace_id=$1 AND journey_id=$2 AND customer_id=$3),(SELECT started_at FROM journey_executions WHERE workspace_id=$1 AND journey_id=$2 AND customer_id=$3 ORDER BY started_at DESC LIMIT 1)`, workspace, m.journey, event.CustomerID).Scan(&active, &previous, &last)
		if err != nil {
			return result, err
		}
		if active {
			continue
		}
		if m.spec.Reentry != nil {
			r := m.spec.Reentry
			if r.Mode == "once" && previous {
				continue
			}
			if r.Mode == "cooldown" && last != nil && now.Before(last.Add(time.Duration(r.CooldownSeconds)*time.Second)) {
				continue
			}
		}
		var id string
		err = tx.QueryRow(ctx, `INSERT INTO journey_executions(workspace_id,journey_id,version_id,customer_id,entry_event_id,current_node_id,started_at,updated_at,exit_event_name) VALUES($1,$2,$3,$4,$5,$6,$7,$7,$8) ON CONFLICT DO NOTHING RETURNING id::text`, workspace, m.journey, m.version, event.CustomerID, event.EventID, start.ID, now, exitName(m.spec)).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO scheduled_jobs(workspace_id,execution_id,node_id,due_at) VALUES($1,$2,$3,$4)`, workspace, id, start.ID, now); err != nil {
			return result, err
		}
		result.Executions = append(result.Executions, id)
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit event: %w", err)
	}
	return result, nil
}

func exitName(w workflow.Workflow) any {
	if w.GlobalExit == nil {
		return nil
	}
	return w.GlobalExit.EventName
}
