package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

// AudienceRun is the outcome of enrolling one audience.
type AudienceRun struct {
	ID       string    `json:"id"`
	SlotAt   time.Time `json:"slot_at"`
	Trigger  string    `json:"trigger"`
	State    string    `json:"state"`
	Checked  int       `json:"checked"`
	Matched  int       `json:"matched"`
	Enrolled int       `json:"enrolled"`
}

// MaxAudience bounds how many customers one run reads.
const MaxAudience = 50000

const audiencePage = 500

// ErrAudienceRunExists means the slot was already claimed by another run.
var ErrAudienceRunExists = errors.New("audience run already exists for this slot")

// RunAudience enrolls every customer who matches the trigger's audience rule
// into one published version. Each customer is handled in a transaction that
// locks the customer row first, like an incoming event, and records an
// audience_entry event with an ID derived from the run and customer.
func RunAudience(ctx context.Context, pool *pgxpool.Pool, workspace, journey, version string, spec workflow.Workflow, slot time.Time, trigger string, now time.Time) (AudienceRun, error) {
	start := spec.Start()
	run := AudienceRun{SlotAt: slot.UTC(), Trigger: trigger, State: "RUNNING"}
	if !start.Config.IsAudience() || start.Config.Filters == nil {
		return run, fmt.Errorf("version does not enroll an audience")
	}
	err := pool.QueryRow(ctx, `INSERT INTO audience_runs(workspace_id,journey_id,version_id,slot_at,trigger,started_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (version_id,slot_at) DO NOTHING RETURNING id::text`, workspace, journey, version, run.SlotAt, trigger, now).Scan(&run.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return run, ErrAudienceRunExists
	}
	if err != nil {
		return run, err
	}
	after := ""
	var failure error
	for run.Checked < MaxAudience && failure == nil {
		rows, err := pool.Query(ctx, `SELECT customer_id FROM customers WHERE workspace_id=$1 AND customer_id>$2 ORDER BY customer_id LIMIT $3`, workspace, after, audiencePage)
		if err != nil {
			failure = err
			break
		}
		ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			failure = err
			break
		}
		if len(ids) == 0 {
			break
		}
		for _, customer := range ids {
			if run.Checked >= MaxAudience {
				break
			}
			run.Checked++
			matched, enrolled, err := enrollAudienceMember(ctx, pool, workspace, journey, version, spec, run.ID, customer, now)
			if err != nil {
				failure = err
				break
			}
			if matched {
				run.Matched++
			}
			if enrolled {
				run.Enrolled++
			}
		}
		after = ids[len(ids)-1]
	}
	run.State = "DONE"
	detail := ""
	if failure != nil {
		run.State = "FAILED"
		detail = failure.Error()
	}
	if _, err := pool.Exec(context.WithoutCancel(ctx), `UPDATE audience_runs SET state=$2,checked=$3,matched=$4,enrolled=$5,detail=NULLIF($6,''),completed_at=now() WHERE id=$1`, run.ID, run.State, run.Checked, run.Matched, run.Enrolled, detail); err != nil {
		return run, err
	}
	return run, failure
}

func enrollAudienceMember(ctx context.Context, pool *pgxpool.Pool, workspace, journey, version string, spec workflow.Workflow, runID, customer string, now time.Time) (matched, enrolled bool, err error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var profileJSON []byte
	err = tx.QueryRow(ctx, `SELECT to_jsonb(c) FROM customers c WHERE workspace_id=$1 AND customer_id=$2 FOR UPDATE`, workspace, customer).Scan(&profileJSON)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	var profile map[string]any
	_ = json.Unmarshal(profileJSON, &profile)
	sum := sha256.Sum256([]byte(runID + "\x00" + customer))
	eventID := "audience_" + hex.EncodeToString(sum[:20])
	properties := map[string]any{"journey_id": journey, "audience_run_id": runID}
	event := map[string]any{"event_id": eventID, "event_name": workflow.AudienceEvent, "occurred_at": now.UTC().Format(time.RFC3339Nano), "properties": properties}
	data := ContextData(profile, event, map[string]any{})
	if !workflow.Evaluate(*spec.Start().Config.Filters, data) {
		return false, false, nil
	}
	canonical, _ := json.Marshal(event)
	fingerprint := sha256.Sum256(canonical)
	tag, err := tx.Exec(ctx, `INSERT INTO customer_events(workspace_id,event_id,customer_id,event_name,occurred_at,properties,payload_hash,status) VALUES($1,$2,$3,$4,$5,$6,$7,'ACCEPTED') ON CONFLICT (workspace_id,event_id) DO NOTHING`, workspace, eventID, customer, workflow.AudienceEvent, now.UTC(), properties, hex.EncodeToString(fingerprint[:]))
	if err != nil {
		return true, false, err
	}
	if tag.RowsAffected() == 0 {
		return true, false, tx.Commit(ctx)
	}
	id, err := enroll(ctx, tx, workspace, journey, version, spec, customer, eventID, now)
	if err != nil {
		return true, false, err
	}
	return true, id != "", tx.Commit(ctx)
}

// AudienceScheduler runs each active audience journey at its scheduled slots.
type AudienceScheduler struct {
	Pool     *pgxpool.Pool
	Log      *slog.Logger
	Interval time.Duration
	Now      func() time.Time
}

func (s *AudienceScheduler) Run(ctx context.Context) {
	interval := s.Interval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := s.Tick(ctx); err != nil && s.Log != nil && ctx.Err() == nil {
			s.Log.Error("audience schedule failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// Tick starts every audience run that is due and returns how many it ran.
// A version's first slot is the first one after it was published, and only
// the latest missed slot runs.
func (s *AudienceScheduler) Tick(ctx context.Context) (int, error) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	rows, err := s.Pool.Query(ctx, `SELECT j.workspace_id::text,j.id::text,v.id::text,v.workflow,GREATEST(v.created_at,COALESCE((SELECT max(r.slot_at) FROM audience_runs r WHERE r.version_id=v.id AND r.trigger='SCHEDULE'),v.created_at)) FROM journeys j JOIN journey_versions v ON v.workspace_id=j.workspace_id AND v.id=j.current_version_id WHERE j.status='Active' AND j.entry_event_name=$1 ORDER BY j.id`, workflow.AudienceEvent)
	if err != nil {
		return 0, err
	}
	type due struct {
		workspace, journey, version string
		spec                        workflow.Workflow
		slot                        time.Time
	}
	pending := []due{}
	for rows.Next() {
		var d due
		var raw []byte
		var anchor time.Time
		if err := rows.Scan(&d.workspace, &d.journey, &d.version, &raw, &anchor); err != nil {
			rows.Close()
			return 0, err
		}
		if json.Unmarshal(raw, &d.spec) != nil || !d.spec.Start().Config.IsAudience() {
			continue
		}
		slot, ok := workflow.LatestAudienceSlot(d.spec.Start().Config, anchor, now)
		if !ok {
			continue
		}
		d.slot = slot
		pending = append(pending, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	ran := 0
	for _, d := range pending {
		_, err := RunAudience(ctx, s.Pool, d.workspace, d.journey, d.version, d.spec, d.slot, "SCHEDULE", now)
		if errors.Is(err, ErrAudienceRunExists) {
			continue
		}
		if err != nil {
			return ran, err
		}
		ran++
	}
	return ran, nil
}
