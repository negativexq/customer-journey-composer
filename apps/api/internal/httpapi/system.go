package httpapi

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"runtime"
)

func registerSystem(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/system/status", func(ctx context.Context, _ *struct{}) (*Result[map[string]any], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		out, err := jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT jsonb_build_object('oldest_metric_delta',(SELECT created_at FROM metric_queue ORDER BY id LIMIT 1),'analytics_processed_at',(SELECT processed_at FROM metric_watermarks WHERE workspace_id=$1),'failed_jobs',COALESCE((SELECT jsonb_agg(to_jsonb(j)) FROM (SELECT id,execution_id,node_id,attempts,manual_retries,last_error,created_at FROM scheduled_jobs WHERE workspace_id=$1 AND state='FAILED' ORDER BY created_at DESC,id DESC LIMIT 20)j),'[]'::jsonb),'failed_or_unknown_deliveries',COALESCE((SELECT jsonb_agg(to_jsonb(d)) FROM (SELECT id,execution_id,node_id,channel,status,attempts,last_error,created_at FROM delivery_jobs WHERE workspace_id=$1 AND status IN ('FAILED','UNKNOWN') ORDER BY created_at DESC,id DESC LIMIT 20)d),'[]'::jsonb))`, who(ctx).WorkspaceID))
		if err != nil {
			return nil, err
		}
		stats := pool.Stat()
		out.Body["pool"] = map[string]any{"max": stats.MaxConns(), "total": stats.TotalConns(), "acquired": stats.AcquiredConns(), "idle": stats.IdleConns(), "empty_acquire_count": stats.EmptyAcquireCount(), "acquire_wait_ms": stats.AcquireDuration().Milliseconds()}
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		out.Body["process"] = map[string]any{"heap_alloc_bytes": memory.HeapAlloc, "heap_sys_bytes": memory.HeapSys, "goroutines": runtime.NumGoroutine()}
		return out, nil
	})
}
