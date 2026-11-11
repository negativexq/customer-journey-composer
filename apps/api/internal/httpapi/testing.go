package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	sim "github.com/negativexq/customer-journey-composer/apps/api/internal/simulation"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type testInput struct {
	Required        *bool         `json:"required,omitempty"`
	Exploratory     bool          `json:"exploratory,omitempty"`
	ScenarioID      string        `json:"scenario_id,omitempty" format:"uuid"`
	JourneyID       string        `json:"journey_id,omitempty" format:"uuid"`
	Version         int           `json:"version,omitempty" minimum:"0"`
	UseCurrentDraft bool          `json:"use_current_draft,omitempty"`
	Scenario        *sim.Scenario `json:"scenario,omitempty"`
}
type testSource struct {
	Exploratory   bool
	JourneyID     string
	VersionID     *string
	DraftRevision *int
	Hash          string
	Workflow      workflow.Workflow
	Scenario      sim.Scenario
	ScenarioID    *string
	ParentID      *string
}
type TestRun struct {
	Acceptance    bool         `json:"acceptance"`
	ID            string       `json:"id"`
	JourneyID     string       `json:"journey_id"`
	VersionID     *string      `json:"version_id,omitempty"`
	DraftRevision *int         `json:"draft_revision,omitempty"`
	WorkflowHash  string       `json:"workflow_hash"`
	Mode          string       `json:"mode"`
	Revision      int          `json:"revision"`
	Scenario      sim.Scenario `json:"scenario"`
	State         sim.State    `json:"state"`
	CreatedAt     time.Time    `json:"created_at"`
	ParentID      *string      `json:"parent_run_id,omitempty"`
}

func sourceForTest(ctx context.Context, pool *pgxpool.Pool, input testInput) (testSource, error) {
	source := testSource{JourneyID: input.JourneyID, Exploratory: input.Exploratory}
	if input.ScenarioID != "" {
		var raw, flow []byte
		var id string
		err := pool.QueryRow(ctx, `SELECT s.id::text,s.journey_id::text,s.version_id::text,s.draft_revision,s.workflow_hash,s.scenario,w.workflow FROM test_scenarios s JOIN sandbox_workflows w ON w.workspace_id=s.workspace_id AND w.workflow_hash=s.workflow_hash WHERE s.workspace_id=$1 AND s.id=$2`, who(ctx).WorkspaceID, input.ScenarioID).Scan(&id, &source.JourneyID, &source.VersionID, &source.DraftRevision, &source.Hash, &raw, &flow)
		if err != nil {
			return source, dbError(err)
		}
		source.ScenarioID = &id
		if err := json.Unmarshal(raw, &source.Scenario); err != nil {
			return source, dbError(err)
		}
		if err := json.Unmarshal(flow, &source.Workflow); err != nil {
			return source, dbError(err)
		}
		if input.Scenario != nil {
			source.Scenario = *input.Scenario
			source.ScenarioID = nil
		}
		if !input.UseCurrentDraft && input.Version == 0 && input.Scenario == nil {
			return source, nil
		}
	}
	if source.JourneyID == "" {
		return source, huma.Error422UnprocessableEntity("Journey ID or saved scenario is required")
	}
	var raw []byte
	if input.ScenarioID != "" && !input.UseCurrentDraft && input.Version == 0 {
		raw, _ = json.Marshal(source.Workflow)
	} else if input.Version > 0 {
		var id string
		err := pool.QueryRow(ctx, `SELECT id::text,workflow FROM journey_versions WHERE workspace_id=$1 AND journey_id=$2 AND version=$3`, who(ctx).WorkspaceID, source.JourneyID, input.Version).Scan(&id, &raw)
		if err != nil {
			return source, dbError(err)
		}
		source.VersionID = &id
		source.DraftRevision = nil
	} else {
		var revision int
		err := pool.QueryRow(ctx, `SELECT draft,revision FROM journeys WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, source.JourneyID).Scan(&raw, &revision)
		if err != nil {
			return source, dbError(err)
		}
		source.DraftRevision = &revision
		source.VersionID = nil
	}
	if err := json.Unmarshal(raw, &source.Workflow); err != nil {
		return source, huma.Error422UnprocessableEntity("Invalid workflow document")
	}
	source.Hash = sim.Hash(source.Workflow)
	if input.Scenario != nil {
		source.Scenario = *input.Scenario
		source.ScenarioID = nil
	}
	if source.Scenario.Name == "" {
		source.Scenario.Name = "Journey test"
	}
	if source.Scenario.StartTime.IsZero() {
		source.Scenario.StartTime = time.Now().UTC().Truncate(time.Microsecond)
	}
	if source.Scenario.Entry.EventName == "" {
		source.Scenario.Entry.EventName = source.Workflow.Start().Config.EventName
	}
	var liveProfile map[string]any
	if source.Scenario.CustomerID != "" && (input.ScenarioID == "" || input.Scenario != nil) {
		var profile []byte
		if err := pool.QueryRow(ctx, `SELECT to_jsonb(c)-'workspace_id' FROM customers c WHERE workspace_id=$1 AND customer_id=$2`, who(ctx).WorkspaceID, source.Scenario.CustomerID).Scan(&profile); err != nil {
			return source, dbError(err)
		}
		_ = json.Unmarshal(profile, &liveProfile)
	}
	if liveProfile != nil {
		for key, value := range source.Scenario.Customer {
			if key == "attributes" {
				attrs, _ := liveProfile[key].(map[string]any)
				if attrs == nil {
					attrs = map[string]any{}
				}
				if overrides, ok := value.(map[string]any); ok {
					for k, v := range overrides {
						attrs[k] = v
					}
					liveProfile[key] = attrs
				}
			} else {
				liveProfile[key] = value
			}
		}
		source.Scenario.Customer = liveProfile
	}
	var limit, window int
	err := pool.QueryRow(ctx, `SELECT frequency_limit,frequency_window_seconds FROM workspace_settings WHERE workspace_id=$1`, who(ctx).WorkspaceID).Scan(&limit, &window)
	if err != nil && err != pgx.ErrNoRows {
		return source, dbError(err)
	}
	if limit == 0 {
		limit = 3
		window = 86400
	}
	if source.Scenario.FrequencyLimit == 0 {
		source.Scenario.FrequencyLimit = limit
	}
	if source.Scenario.FrequencyWindowSeconds == 0 {
		source.Scenario.FrequencyWindowSeconds = window
	}
	return source, nil
}
func saveSnapshot(ctx context.Context, tx pgx.Tx, workspace string, source testSource) error {
	data, _ := json.Marshal(source.Workflow)
	_, err := tx.Exec(ctx, `INSERT INTO sandbox_workflows(workspace_id,workflow_hash,workflow) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, workspace, source.Hash, data)
	return err
}
func createTestRun(ctx context.Context, pool *pgxpool.Pool, source testSource) (*Result[TestRun], error) {
	state, _, scenario, err := sim.New(source.Workflow, source.Scenario)
	if err != nil {
		return nil, huma.Error422UnprocessableEntity(err.Error())
	}
	source.Scenario = scenario
	mode := "FULL"
	if scenario.Mode == "single_node" {
		mode = "SINGLE_NODE"
	}
	workspace := who(ctx).WorkspaceID
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, dbError(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT 1 FROM journeys WHERE workspace_id=$1 AND id=$2 FOR SHARE`, workspace, source.JourneyID); err != nil {
		return nil, dbError(err)
	}
	if err := saveSnapshot(ctx, tx, workspace, source); err != nil {
		return nil, dbError(err)
	}
	scenarioJSON, _ := json.Marshal(scenario)
	stateJSON, _ := json.Marshal(state)
	var customer *string
	if scenario.CustomerID != "" {
		customer = &scenario.CustomerID
	}
	out, err := jsonRow[TestRun](tx.QueryRow(ctx, `INSERT INTO test_runs(workspace_id,journey_id,version_id,draft_revision,scenario_id,parent_run_id,workflow_hash,source_customer_id,mode,scenario,state,execution_state,virtual_now,created_by,acceptance) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) RETURNING to_jsonb(test_runs)-'workspace_id'`, workspace, source.JourneyID, source.VersionID, source.DraftRevision, source.ScenarioID, source.ParentID, source.Hash, customer, mode, scenarioJSON, stateJSON, state.Status, state.Now, who(ctx).ID, !source.Exploratory))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, dbError(err)
	}
	return out, nil
}
func persistTestMessages(ctx context.Context, tx pgx.Tx, workspace, run, journey string, version *string, mode string, state *sim.State) error {
	records := []map[string]any{}
	for i := range state.Messages {
		message := &state.Messages[i]
		if !message.Dirty {
			continue
		}
		rendered := any(map[string]any{})
		if !message.PayloadPersisted {
			rendered = message.Rendered
		}
		at := message.At
		if message.AcceptedAt != nil {
			at = *message.AcceptedAt
		}
		records = append(records, map[string]any{"message_key": message.ID, "execution_index": message.Execution, "customer_id": message.CustomerID, "source_node": message.NodeID, "node_label": message.NodeLabel, "channel": message.Channel, "status": message.Status, "simulated_at": at, "scheduled_at": message.At, "message": rendered, "decision": message.Decision, "attempt_history": message.Attempts, "provider_response": message.ProviderResponse})
	}
	if len(records) == 0 {
		return nil
	}
	payload, _ := json.Marshal(records)
	inboxMode := "SIMULATED"
	if mode == "SINGLE_NODE" {
		inboxMode = "SINGLE_NODE"
	}
	_, err := tx.Exec(ctx, `INSERT INTO test_messages(workspace_id,test_run_id,journey_id,version_id,mode,message_key,execution_index,customer_id,source_node,node_label,channel,status,simulated_at,scheduled_at,message,decision,attempt_history,provider_response)
 SELECT $1,$2,$3,$4,$5,r.message_key,r.execution_index,r.customer_id,r.source_node,r.node_label,r.channel,r.status,r.simulated_at,r.scheduled_at,r.message,r.decision,r.attempt_history,r.provider_response FROM jsonb_to_recordset($6) AS r(message_key text,execution_index int,customer_id text,source_node text,node_label text,channel text,status text,simulated_at timestamptz,scheduled_at timestamptz,message jsonb,decision jsonb,attempt_history jsonb,provider_response text)
 ON CONFLICT(test_run_id,message_key) DO UPDATE SET status=excluded.status,simulated_at=excluded.simulated_at,decision=excluded.decision,attempt_history=excluded.attempt_history,provider_response=excluded.provider_response`, workspace, run, journey, version, inboxMode, payload)
	if err != nil {
		return err
	}
	for i := range state.Messages {
		message := &state.Messages[i]
		if !message.Dirty {
			continue
		}
		message.PayloadPersisted = true
		message.Dirty = false
		message.Rendered = messaging.Message{Channel: message.Rendered.Channel, To: message.Rendered.To, Encoding: message.Rendered.Encoding, Units: message.Rendered.Units, Segments: message.Rendered.Segments, TemplateID: message.Rendered.TemplateID}
	}
	return nil
}
func registerTesting(api huma.API, pool *pgxpool.Pool) {
	huma.Post(api, "/api/v1/test-scenarios", func(ctx context.Context, in *struct{ Body testInput }) (*Result[map[string]any], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		source, err := sourceForTest(ctx, pool, in.Body)
		if err != nil {
			return nil, err
		}
		_, _, scenario, err := sim.New(source.Workflow, source.Scenario)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		source.Scenario = scenario
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if _, err := tx.Exec(ctx, `SELECT 1 FROM journeys WHERE workspace_id=$1 AND id=$2 FOR SHARE`, who(ctx).WorkspaceID, source.JourneyID); err != nil {
			return nil, dbError(err)
		}
		if err := saveSnapshot(ctx, tx, who(ctx).WorkspaceID, source); err != nil {
			return nil, dbError(err)
		}
		data, _ := json.Marshal(scenario)
		var customer *string
		if scenario.CustomerID != "" {
			customer = &scenario.CustomerID
		}
		out, err := jsonRow[map[string]any](tx.QueryRow(ctx, `INSERT INTO test_scenarios(workspace_id,journey_id,version_id,draft_revision,workflow_hash,source_customer_id,name,scenario,created_by,required) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING jsonb_build_object('id',id,'name',name,'workflow_hash',workflow_hash,'created_at',created_at,'required',required)`, who(ctx).WorkspaceID, source.JourneyID, source.VersionID, source.DraftRevision, source.Hash, customer, scenario.Name, data, who(ctx).ID, in.Body.Required == nil || *in.Body.Required))
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return out, nil
	})
	huma.Patch(api, "/api/v1/test-scenarios/{id}/requirement", func(ctx context.Context, in *struct {
		ID   string `path:"id" format:"uuid"`
		Body struct {
			Required bool `json:"required"`
		}
	}) (*Result[map[string]any], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		var journey string
		if err = tx.QueryRow(ctx, `SELECT journey_id::text FROM test_scenarios WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.ID).Scan(&journey); err != nil {
			return nil, dbError(err)
		}
		if _, err = tx.Exec(ctx, `SELECT 1 FROM journeys WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, who(ctx).WorkspaceID, journey); err != nil {
			return nil, dbError(err)
		}
		out, err := jsonRow[map[string]any](tx.QueryRow(ctx, `UPDATE test_scenarios SET required=$3 WHERE workspace_id=$1 AND id=$2 RETURNING jsonb_build_object('id',id,'required',required)`, who(ctx).WorkspaceID, in.ID, in.Body.Required))
		if err != nil {
			return nil, err
		}
		if err = audit(ctx, tx, "test_scenario.requirement", &in.ID, map[string]any{"required": in.Body.Required}); err != nil {
			return nil, dbError(err)
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return out, nil
	})
	huma.Get(api, "/api/v1/test-scenarios", func(ctx context.Context, _ *struct{}) (*Result[[]map[string]any], error) {
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(s)),'[]'::jsonb) FROM (SELECT id,journey_id,version_id,draft_revision,name,workflow_hash,required,created_at FROM test_scenarios WHERE workspace_id=$1 ORDER BY created_at DESC,id LIMIT 100) s`, who(ctx).WorkspaceID))
	})
	huma.Post(api, "/api/v1/test-runs", func(ctx context.Context, in *struct{ Body testInput }) (*Result[TestRun], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		source, err := sourceForTest(ctx, pool, in.Body)
		if err != nil {
			return nil, err
		}
		return createTestRun(ctx, pool, source)
	})
	huma.Get(api, "/api/v1/test-runs", func(ctx context.Context, in *struct {
		JourneyID string `query:"journey_id"`
		Limit     int    `query:"limit" default:"50" minimum:"1" maximum:"200"`
	}) (*Result[[]map[string]any], error) {
		where := `workspace_id=$1`
		args := []any{who(ctx).WorkspaceID}
		if in.JourneyID != "" {
			where += ` AND journey_id=$2::uuid`
			args = append(args, in.JourneyID)
		}
		args = append(args, in.Limit)
		query := fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(r)),'[]'::jsonb) FROM (SELECT id,journey_id,version_id,mode,workflow_hash,execution_state,result,finalized,virtual_now,message_count,created_at FROM test_runs WHERE %s ORDER BY created_at DESC,id LIMIT $%d) r`, where, len(args))
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, query, args...))
	})
	huma.Get(api, "/api/v1/test-runs/{id}", func(ctx context.Context, in *journeyID) (*Result[TestRun], error) {
		return jsonRow[TestRun](pool.QueryRow(ctx, `SELECT to_jsonb(r)-'workspace_id' FROM test_runs r WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.ID))
	})
	huma.Post(api, "/api/v1/test-runs/{id}/advance", func(ctx context.Context, in *struct {
		ID   string `path:"id" format:"uuid"`
		Body struct {
			Operation string `json:"operation" enum:"run,step,advance,until_next,complete,pause,resume"`
			Seconds   int64  `json:"seconds,omitempty" minimum:"0" maximum:"31536000"`
			Revision  int    `json:"revision" minimum:"1"`
		}
	}) (*Result[TestRun], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		var stateJSON, scenarioJSON, flowJSON []byte
		var revision int
		var journey, mode string
		var version *string
		err = tx.QueryRow(ctx, `SELECT r.state,r.scenario,w.workflow,r.revision,r.journey_id::text,r.version_id::text,r.mode FROM test_runs r JOIN sandbox_workflows w ON w.workspace_id=r.workspace_id AND w.workflow_hash=r.workflow_hash WHERE r.workspace_id=$1 AND r.id=$2 FOR UPDATE OF r`, who(ctx).WorkspaceID, in.ID).Scan(&stateJSON, &scenarioJSON, &flowJSON, &revision, &journey, &version, &mode)
		if err != nil {
			return nil, dbError(err)
		}
		if revision != in.Body.Revision {
			return nil, huma.Error409Conflict("Test run changed; reload before advancing")
		}
		var state sim.State
		var scenario sim.Scenario
		var spec workflow.Workflow
		if json.Unmarshal(stateJSON, &state) != nil || json.Unmarshal(scenarioJSON, &scenario) != nil || json.Unmarshal(flowJSON, &spec) != nil {
			return nil, huma.Error500InternalServerError("Invalid saved simulation state")
		}
		if scenario.Mode == "single_node" {
			spec, err = sim.SingleNode(spec, scenario.NodeID, scenario.Entry.EventName)
			if err != nil {
				return nil, huma.Error422UnprocessableEntity(err.Error())
			}
		}
		if err := sim.Advance(spec, scenario, &state, in.Body.Operation, in.Body.Seconds); err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if err := persistTestMessages(ctx, tx, who(ctx).WorkspaceID, in.ID, journey, version, mode, &state); err != nil {
			return nil, dbError(err)
		}
		data, _ := json.Marshal(state)
		if len(data) > 8<<20 {
			return nil, huma.Error422UnprocessableEntity("Simulation state exceeds 8 MiB")
		}
		out, err := jsonRow[TestRun](tx.QueryRow(ctx, `UPDATE test_runs SET state=$3,execution_state=$4,result=$5,finalized=$6,virtual_now=$7,message_count=$8,revision=revision+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING to_jsonb(test_runs)-'workspace_id'`, who(ctx).WorkspaceID, in.ID, data, state.Status, state.Result, state.Finalized, state.Now, len(state.Messages)))
		if err != nil {
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return out, nil
	})
	huma.Post(api, "/api/v1/test-runs/{id}/replay", func(ctx context.Context, in *journeyID) (*Result[TestRun], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		var source testSource
		var scene, flow []byte
		err := pool.QueryRow(ctx, `SELECT r.journey_id::text,r.version_id::text,r.draft_revision,r.workflow_hash,r.scenario,w.workflow,r.scenario_id::text,NOT r.acceptance FROM test_runs r JOIN sandbox_workflows w ON w.workspace_id=r.workspace_id AND w.workflow_hash=r.workflow_hash WHERE r.workspace_id=$1 AND r.id=$2`, who(ctx).WorkspaceID, in.ID).Scan(&source.JourneyID, &source.VersionID, &source.DraftRevision, &source.Hash, &scene, &flow, &source.ScenarioID, &source.Exploratory)
		if err != nil {
			return nil, dbError(err)
		}
		_ = json.Unmarshal(scene, &source.Scenario)
		_ = json.Unmarshal(flow, &source.Workflow)
		source.ParentID = &in.ID
		return createTestRun(ctx, pool, source)
	})
	huma.Post(api, "/api/v1/preview/journey", func(ctx context.Context, in *struct{ Body testInput }) (*Result[sim.State], error) {
		source, err := sourceForTest(ctx, pool, in.Body)
		if err != nil {
			return nil, err
		}
		state, spec, scenario, err := sim.New(source.Workflow, source.Scenario)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if err := sim.Advance(spec, scenario, &state, "complete", 0); err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		return &Result[sim.State]{Body: state}, nil
	})
}
