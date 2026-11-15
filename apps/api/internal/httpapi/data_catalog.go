package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/semantic"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type catalogReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadDataCatalog(ctx context.Context, db catalogReader) ([]semantic.Entry, error) {
	rows, err := db.Query(ctx, `SELECT key,kind,event_name,path,label,description,data_type,aliases,enabled,source FROM workspace_data_catalog WHERE workspace_id=$1 ORDER BY key LIMIT 1001`, who(ctx).WorkspaceID)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	entries := semantic.Builtins()
	count := 0
	for rows.Next() {
		var e semantic.Entry
		if err = rows.Scan(&e.Key, &e.Kind, &e.EventName, &e.Path, &e.Label, &e.Description, &e.DataType, &e.Aliases, &e.Enabled, &e.Source); err != nil {
			return nil, dbError(err)
		}
		entries = append(entries, e)
		count++
	}
	if rows.Err() != nil {
		return nil, dbError(rows.Err())
	}
	if count > 1000 {
		return nil, huma.Error422UnprocessableEntity("Catalog limit exceeded")
	}
	declared := map[string]bool{}
	for _, e := range entries {
		declared[e.Key] = true
	}
	for _, e := range semantic.EngagementEntries() {
		if !declared[e.Key] {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

type catalogWrite struct {
	Body struct {
		Entries []semantic.Entry `json:"entries" minItems:"1" maxItems:"500"`
	}
}
type catalogResolve struct {
	Body struct {
		Workflow  workflow.Workflow  `json:"workflow"`
		Bindings  []semantic.Binding `json:"bindings" maxItems:"500"`
		Confirmed []string           `json:"confirmed" maxItems:"500"`
	}
}
type catalogResolution struct {
	Workflow  workflow.Workflow `json:"workflow"`
	Grounding semantic.Report   `json:"grounding"`
}

func writeCatalog(ctx context.Context, pool *pgxpool.Pool, entries []semantic.Entry, observed bool) error {
	for i := range entries {
		if err := entries[i].Validate(); err != nil {
			return huma.Error422UnprocessableEntity(err.Error())
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return dbError(err)
	}
	defer tx.Rollback(ctx)
	// Serialize design-time writes without blocking runtime foreign-key KEY SHARE locks.
	var workspace string
	if err = tx.QueryRow(ctx, `SELECT id FROM workspaces WHERE id=$1 FOR NO KEY UPDATE`, who(ctx).WorkspaceID).Scan(&workspace); err != nil {
		return dbError(err)
	}
	var count int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM workspace_data_catalog WHERE workspace_id=$1`, workspace).Scan(&count); err != nil {
		return dbError(err)
	}
	for _, e := range entries {
		aliases, _ := json.Marshal(e.Aliases)
		source := "declared"
		if observed {
			source = "observed"
		}
		query := `INSERT INTO workspace_data_catalog(workspace_id,key,kind,event_name,path,label,description,data_type,aliases,enabled,source) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(workspace_id,key) DO UPDATE SET label=EXCLUDED.label,description=EXCLUDED.description,data_type=EXCLUDED.data_type,aliases=EXCLUDED.aliases,enabled=EXCLUDED.enabled,source='declared',updated_at=now()`
		if observed {
			query = `INSERT INTO workspace_data_catalog(workspace_id,key,kind,event_name,path,label,description,data_type,aliases,enabled,source) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(workspace_id,key) DO UPDATE SET data_type=CASE WHEN workspace_data_catalog.data_type=EXCLUDED.data_type THEN workspace_data_catalog.data_type ELSE 'mixed' END,updated_at=now() WHERE workspace_data_catalog.source='observed'`
		}
		if _, err = tx.Exec(ctx, query, workspace, e.Key, e.Kind, e.EventName, e.Path, e.Label, e.Description, e.DataType, aliases, e.Enabled, source); err != nil {
			return dbError(err)
		}
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM workspace_data_catalog WHERE workspace_id=$1`, workspace).Scan(&count); err != nil {
		return dbError(err)
	}
	if count > 1000 {
		return huma.Error422UnprocessableEntity("Maximum 1,000 custom catalog entries per workspace")
	}
	if err = audit(ctx, tx, "data_catalog.updated", nil, map[string]any{"entries": len(entries), "discovery": observed}); err != nil {
		return dbError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return dbError(err)
	}
	return nil
}
func registerDataCatalog(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/data-catalog", func(ctx context.Context, _ *struct{}) (*Result[[]semantic.Entry], error) {
		entries, err := loadDataCatalog(ctx, pool)
		return &Result[[]semantic.Entry]{Body: entries}, err
	})
	huma.Put(api, "/api/v1/data-catalog", func(ctx context.Context, in *catalogWrite) (*Result[[]semantic.Entry], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		if err := writeCatalog(ctx, pool, in.Body.Entries, false); err != nil {
			return nil, err
		}
		entries, err := loadDataCatalog(ctx, pool)
		return &Result[[]semantic.Entry]{Body: entries}, err
	})
	huma.Post(api, "/api/v1/data-catalog/resolve", func(ctx context.Context, in *catalogResolve) (*Result[catalogResolution], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		entries, err := loadDataCatalog(ctx, pool)
		if err != nil {
			return nil, err
		}
		w, err := semantic.Resolve(in.Body.Workflow, entries, in.Body.Bindings)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		return &Result[catalogResolution]{Body: catalogResolution{w, semantic.Analyze(w, entries, in.Body.Confirmed)}}, nil
	})
	huma.Post(api, "/api/v1/data-catalog/discover", func(ctx context.Context, _ *struct{}) (*Result[[]semantic.Entry], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		entries, err := discoverCatalog(ctx, pool)
		if err != nil {
			return nil, err
		}
		if len(entries) > 0 {
			if err = writeCatalog(ctx, pool, entries, true); err != nil {
				return nil, err
			}
		}
		result, err := loadDataCatalog(ctx, pool)
		return &Result[[]semantic.Entry]{Body: result}, err
	})
}

// Explicit, bounded discovery uses existing recent-row indexes and never runs
// during ingestion or generation. Values stay local; only names/types persist.
func discoverCatalog(ctx context.Context, pool *pgxpool.Pool) ([]semantic.Entry, error) {
	found := map[string]semantic.Entry{}
	add := func(e semantic.Entry) {
		if len(found) >= 500 {
			if _, exists := found[semantic.Key(e.Kind, e.EventName, e.Path)]; !exists {
				return
			}
		}
		e.Enabled = true
		e.Source = "observed"
		if e.Validate() != nil {
			return
		}
		if old, exists := found[e.Key]; exists && old.DataType != e.DataType {
			e.DataType = "mixed"
		}
		found[e.Key] = e
	}
	var fields func(map[string]any, string, string, string, int)
	fields = func(data map[string]any, prefix, kind, event string, depth int) {
		if depth > 4 {
			return
		}
		n := 0
		keys := []string{}
		for key := range data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := data[key]
			n++
			if n > 100 {
				return
			}
			path := prefix + "." + key
			if object, ok := value.(map[string]any); ok {
				fields(object, path, kind, event, depth+1)
				continue
			}
			typ := ""
			switch value.(type) {
			case string:
				typ = "string"
			case float64:
				typ = "number"
			case bool:
				typ = "boolean"
			}
			if typ != "" {
				add(semantic.Entry{Kind: kind, EventName: event, Path: path, Label: semantic.Humanize(key), DataType: typ, Aliases: []string{}})
			}
		}
	}
	rows, err := pool.Query(ctx, `SELECT event_name,properties FROM (SELECT event_name,properties FROM customer_events WHERE workspace_id=$1 ORDER BY received_at DESC,event_id DESC LIMIT 100) recent WHERE octet_length(properties::text)<=32768`, who(ctx).WorkspaceID)
	if err != nil {
		return nil, dbError(err)
	}
	for rows.Next() {
		var name string
		var data map[string]any
		if err = rows.Scan(&name, &data); err != nil {
			rows.Close()
			return nil, dbError(err)
		}
		add(semantic.Entry{Kind: "event", EventName: name, Label: semantic.Humanize(name), DataType: "event", Aliases: []string{}})
		fields(data, "event.properties", "event_field", name, 0)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, dbError(err)
	}
	rows, err = pool.Query(ctx, `SELECT attributes FROM (SELECT attributes FROM customers WHERE workspace_id=$1 ORDER BY created_at DESC,customer_id DESC LIMIT 100) recent WHERE octet_length(attributes::text)<=32768`, who(ctx).WorkspaceID)
	if err != nil {
		return nil, dbError(err)
	}
	defer rows.Close()
	for rows.Next() {
		var data map[string]any
		if err = rows.Scan(&data); err != nil {
			return nil, dbError(err)
		}
		fields(data, "customer.attributes", "customer_field", "", 0)
	}
	if rows.Err() != nil {
		return nil, dbError(rows.Err())
	}
	out := []semantic.Entry{}
	for _, e := range found {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
func groundingRequired(ctx context.Context, db catalogReader, w workflow.Workflow, confirmed []string) error {
	entries, err := loadDataCatalog(ctx, db)
	if err != nil {
		return err
	}
	report := semantic.Analyze(w, entries, confirmed)
	if !report.Ready {
		names := []string{}
		for _, m := range report.Mappings {
			if m.Status != "matched" && m.Status != "confirmed" {
				names = append(names, m.Label)
			}
		}
		return huma.Error422UnprocessableEntity(fmt.Sprintf("Review data mappings before approval: %s", strings.Join(names, ", ")))
	}
	return nil
}
