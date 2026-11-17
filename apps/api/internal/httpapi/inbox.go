package httpapi

import (
	"context"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

func registerInbox(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/test-inbox", func(ctx context.Context, in *struct {
		RunID      string    `query:"test_run_id" format:"uuid"`
		CustomerID string    `query:"customer_id" maxLength:"128"`
		JourneyID  string    `query:"journey_id" format:"uuid"`
		Channel    string    `query:"channel" enum:"email,sms,push"`
		Status     string    `query:"status" enum:"SIMULATED,BLOCKED,SKIPPED,FAILED,TEST_ACCEPTED,TEST_PENDING"`
		Before     time.Time `query:"before"`
		BeforeID   string    `query:"before_id" format:"uuid"`
		Limit      int       `query:"limit" default:"50" minimum:"1" maximum:"200"`
	}) (*Result[[]map[string]any], error) {
		where := "m.workspace_id=$1"
		args := []any{who(ctx).WorkspaceID}
		add := func(column, value string) {
			if value != "" {
				args = append(args, value)
				where += fmt.Sprintf(" AND m.%s=$%d", column, len(args))
			}
		}
		add("test_run_id", in.RunID)
		add("customer_id", in.CustomerID)
		add("journey_id", in.JourneyID)
		add("channel", in.Channel)
		add("status", in.Status)
		if !in.Before.IsZero() {
			args = append(args, in.Before)
			if in.BeforeID != "" {
				args = append(args, in.BeforeID)
				where += fmt.Sprintf(" AND (m.created_at,m.id)<($%d,$%d::uuid)", len(args)-1, len(args))
			} else {
				where += fmt.Sprintf(" AND m.created_at<$%d", len(args))
			}
		}
		args = append(args, in.Limit)
		// Project metadata before joining; never detoast message bodies for a list.
		query := fmt.Sprintf(`SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY r.created_at DESC,r.id DESC),'[]'::jsonb) FROM (SELECT m.id,m.test_run_id,m.delivery_id,m.journey_id,m.customer_id,m.source_node,m.node_label,m.channel,m.mode,m.status,m.simulated_at,m.created_at,j.name AS journey_name,v.version FROM (SELECT id,test_run_id,delivery_id,journey_id,version_id,customer_id,source_node,node_label,channel,mode,status,simulated_at,created_at FROM test_messages m WHERE %s ORDER BY created_at DESC,id DESC LIMIT $%d) m LEFT JOIN journeys j ON j.id=m.journey_id LEFT JOIN journey_versions v ON v.id=m.version_id) r`, where, len(args))
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, query, args...))
	})
	huma.Get(api, "/api/v1/test-messages/{id}", func(ctx context.Context, in *journeyID) (*Result[map[string]any], error) {
		return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT (to_jsonb(m)-'workspace_id')||jsonb_build_object('journey_name',j.name,'version',v.version,'workflow_hash',r.workflow_hash,'trace_id',COALESCE(r.id,m.delivery_id),'attempt_history',CASE WHEN m.delivery_id IS NOT NULL THEN COALESCE((SELECT jsonb_agg(to_jsonb(a)-'workspace_id' ORDER BY a.attempt) FROM delivery_attempts a WHERE a.delivery_id=m.delivery_id),'[]'::jsonb) ELSE m.attempt_history END,'provider',d.provider,'engagement',`+engagementJSON+`,'provider_status',d.status,'provider_response',COALESCE(d.last_error,m.provider_response),'execution_trace',COALESCE(r.state->'trace','[]'::jsonb),'scenario',r.scenario) FROM test_messages m LEFT JOIN test_runs r ON r.workspace_id=m.workspace_id AND r.id=m.test_run_id LEFT JOIN journeys j ON j.id=m.journey_id LEFT JOIN journey_versions v ON v.id=m.version_id LEFT JOIN delivery_jobs d ON d.id=m.delivery_id WHERE m.workspace_id=$1 AND m.id=$2`, who(ctx).WorkspaceID, in.ID))
	})
	// Compare the same logical node across the explicitly selected replay parent.
	huma.Get(api, "/api/v1/test-messages/{id}/previous", func(ctx context.Context, in *journeyID) (*Result[map[string]any], error) {
		return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT jsonb_build_object('id',p.id,'status',p.status,'message',p.message,'decision',p.decision,'attempt_history',p.attempt_history,'simulated_at',p.simulated_at) FROM test_messages m JOIN test_runs r ON r.workspace_id=m.workspace_id AND r.id=m.test_run_id JOIN test_messages p ON p.workspace_id=m.workspace_id AND p.test_run_id=r.parent_run_id AND p.message_key=m.message_key WHERE m.workspace_id=$1 AND m.id=$2`, who(ctx).WorkspaceID, in.ID))
	})
}
