package httpapi

import (
	"context"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/analytics"
	"strings"
	"time"
)

type journeyListInput struct {
	Search    string    `query:"search" maxLength:"160"`
	Status    string    `query:"status" enum:"Draft,Active,Paused,Sunset,Completed,Archived"`
	Sort      string    `query:"sort" enum:"updated,name" default:"updated"`
	Limit     int       `query:"limit" default:"50" minimum:"1" maximum:"200"`
	Offset    int       `query:"offset" minimum:"0" maximum:"1000"`
	Before    time.Time `query:"before"`
	AfterName string    `query:"after_name" maxLength:"160"`
	BeforeID  string    `query:"before_id" format:"uuid"`
}

func listJourneys(ctx context.Context, pool *pgxpool.Pool, in *journeyListInput) (*Result[[]Journey], error) {
	args := []any{who(ctx).WorkspaceID}
	where := "workspace_id=$1"
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if in.Search != "" {
		escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(strings.TrimSpace(in.Search))
		where += ` AND lower(name) COLLATE "C" LIKE lower(` + add(escaped) + `::text)||'%'`
	}
	if in.Status != "" {
		where += " AND status=" + add(in.Status)
	}
	order := "updated_at DESC,id DESC"
	if in.Sort == "name" {
		order = `lower(name) COLLATE "C",id`
		if in.AfterName != "" {
			if in.BeforeID == "" {
				return nil, huma.Error422UnprocessableEntity("before_id is required with after_name")
			}
			where += ` AND (lower(name) COLLATE "C",id)>(lower(` + add(in.AfterName) + `::text) COLLATE "C",` + add(in.BeforeID) + `::uuid)`
		}
	} else if !in.Before.IsZero() {
		if in.BeforeID == "" {
			return nil, huma.Error422UnprocessableEntity("before_id is required with before")
		}
		where += ` AND (updated_at,id)<(` + add(in.Before) + `,` + add(in.BeforeID) + `::uuid)`
	}
	limit := add(in.Limit)
	offset := add(in.Offset)
	zero := add(analytics.ZeroID)
	// One bounded query: fetch the page before indexed counter lookups. No draft JSON.
	aggOrder := "r.updated_at DESC,r.id DESC"
	if in.Sort == "name" {
		aggOrder = `lower(r.name) COLLATE "C",r.id`
	}
	query := `SELECT COALESCE(jsonb_agg(to_jsonb(r) ORDER BY ` + aggOrder + `),'[]'::jsonb) FROM (SELECT j.*,v.version,jsonb_build_object('entries',COALESCE(m.entries,0),'active',COALESCE(m.active,0),'completed',COALESCE(m.completed,0),'converted',COALESCE(m.converted,0)) AS stats FROM (SELECT id,name,description,tags,status,revision,current_version_id,updated_at FROM journeys WHERE ` + where + ` ORDER BY ` + order + ` LIMIT ` + limit + ` OFFSET ` + offset + `) j LEFT JOIN journey_versions v ON v.id=j.current_version_id LEFT JOIN LATERAL (SELECT (sum(value) FILTER(WHERE metric='entries'))::bigint AS entries,(sum(value) FILTER(WHERE metric IN ('RUNNING','WAITING')))::bigint AS active,(sum(value) FILTER(WHERE metric='COMPLETED'))::bigint AS completed,(sum(value) FILTER(WHERE metric='converted'))::bigint AS converted FROM metric_totals WHERE workspace_id=$1 AND journey_id=j.id AND version_id=` + zero + `::uuid AND dimension='execution')m ON true) r`
	return jsonRow[[]Journey](pool.QueryRow(ctx, query, args...))
}
