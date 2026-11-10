package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	engine "github.com/ofk/customer-journey-composer/apps/api/internal/runtime"
)

type Customer struct {
	CustomerID string          `json:"customer_id"`
	ExternalID *string         `json:"external_id,omitempty"`
	Email      *string         `json:"email,omitempty"`
	Phone      *string         `json:"phone,omitempty"`
	Locale     string          `json:"locale"`
	Timezone   string          `json:"timezone"`
	Attributes map[string]any  `json:"attributes"`
	Consents   map[string]bool `json:"consents"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}
type customerBody struct {
	CustomerID string          `json:"customer_id,omitempty" maxLength:"128"`
	ExternalID *string         `json:"external_id,omitempty" maxLength:"128"`
	Email      *string         `json:"email,omitempty" maxLength:"254"`
	Phone      *string         `json:"phone,omitempty" maxLength:"32"`
	Locale     string          `json:"locale,omitempty" default:"en" maxLength:"20"`
	Timezone   string          `json:"timezone,omitempty" default:"UTC"`
	Attributes map[string]any  `json:"attributes,omitempty"`
	Consents   map[string]bool `json:"consents,omitempty"`
}
type customerID struct {
	ID string `path:"id" maxLength:"128"`
}

func eventError(err error) error {
	var input *engine.InputError
	if errors.As(err, &input) {
		if input.Conflict {
			return huma.Error409Conflict(input.Message)
		}
		return huma.Error422UnprocessableEntity(input.Message)
	}
	return dbError(err)
}
func registerCustomers(api huma.API, pool *pgxpool.Pool) {
	save := func(ctx context.Context, body customerBody, update bool) (*Result[Customer], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		if body.CustomerID == "" {
			body.CustomerID = randomToken()[:24]
		}
		if body.Locale == "" {
			body.Locale = "en"
		}
		if body.Timezone == "" {
			body.Timezone = "UTC"
		}
		if _, err := time.LoadLocation(body.Timezone); err != nil {
			return nil, huma.Error422UnprocessableEntity("Unknown timezone")
		}
		if body.Email != nil {
			address, err := mail.ParseAddress(*body.Email)
			if err != nil || address.Address != *body.Email {
				return nil, huma.Error422UnprocessableEntity("Valid customer email required")
			}
		}
		if body.Attributes == nil {
			body.Attributes = map[string]any{}
		}
		if body.Consents == nil {
			body.Consents = map[string]bool{"email": false, "sms": false}
		}
		raw, _ := json.Marshal(body.Attributes)
		if len(raw) > 65536 {
			return nil, huma.Error422UnprocessableEntity("Attributes exceed 64 KiB")
		}
		if update {
			return jsonRow[Customer](pool.QueryRow(ctx, `UPDATE customers SET external_id=$3,email=$4,phone=$5,locale=$6,timezone=$7,attributes=$8,consents=$9,updated_at=now() WHERE workspace_id=$1 AND customer_id=$2 RETURNING to_jsonb(customers)-'workspace_id'`, who(ctx).WorkspaceID, body.CustomerID, body.ExternalID, body.Email, body.Phone, body.Locale, body.Timezone, body.Attributes, body.Consents))
		}
		return jsonRow[Customer](pool.QueryRow(ctx, `INSERT INTO customers(workspace_id,customer_id,external_id,email,phone,locale,timezone,attributes,consents) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING to_jsonb(customers)-'workspace_id'`, who(ctx).WorkspaceID, body.CustomerID, body.ExternalID, body.Email, body.Phone, body.Locale, body.Timezone, body.Attributes, body.Consents))
	}
	huma.Post(api, "/api/v1/customers", func(ctx context.Context, in *struct{ Body customerBody }) (*Result[Customer], error) {
		return save(ctx, in.Body, false)
	})
	huma.Put(api, "/api/v1/customers/{id}", func(ctx context.Context, in *struct {
		ID   string `path:"id"`
		Body customerBody
	}) (*Result[Customer], error) {
		in.Body.CustomerID = in.ID
		return save(ctx, in.Body, true)
	})

	huma.Get(api, "/api/v1/customers", func(ctx context.Context, in *struct {
		Search   string    `query:"search" maxLength:"128"`
		Limit    int       `query:"limit" default:"50" minimum:"1" maximum:"200"`
		Before   time.Time `query:"before"`
		BeforeID string    `query:"before_id" maxLength:"128"`
	}) (*Result[[]Customer], error) {
		args := []any{who(ctx).WorkspaceID}
		query := `SELECT customer_id,external_id,email,phone,locale,timezone,created_at,updated_at FROM customers WHERE workspace_id=$1`
		if search := strings.TrimSpace(in.Search); search != "" {
			prefix := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(search))
			args = append(args, search, prefix)
			query += ` AND (customer_id=$2 OR lower(email) LIKE $3||'%' OR external_id=$2)`
		}
		if !in.Before.IsZero() {
			args = append(args, in.Before, in.BeforeID)
			query += fmt.Sprintf(` AND (created_at,customer_id)<($%d,$%d)`, len(args)-1, len(args))
		}
		args = append(args, in.Limit)
		query += fmt.Sprintf(` ORDER BY created_at DESC,customer_id DESC LIMIT $%d`, len(args))
		return jsonRow[[]Customer](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(c)),'[]'::jsonb) FROM (`+query+`) c`, args...))
	})
	huma.Get(api, "/api/v1/customers/{id}", func(ctx context.Context, in *customerID) (*Result[Customer], error) {
		return jsonRow[Customer](pool.QueryRow(ctx, `SELECT to_jsonb(c)-'workspace_id' FROM customers c WHERE workspace_id=$1 AND customer_id=$2`, who(ctx).WorkspaceID, in.ID))
	})
	huma.Get(api, "/api/v1/customers/{id}/events", func(ctx context.Context, in *struct {
		ID    string `path:"id"`
		Limit int    `query:"limit" default:"50" minimum:"1" maximum:"200"`
	}) (*Result[[]map[string]any], error) {
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(e)-'workspace_id'-'payload_hash'),'[]'::jsonb) FROM (SELECT event_id,customer_id,event_name,occurred_at,received_at,status,duplicate_count FROM customer_events WHERE workspace_id=$1 AND customer_id=$2 ORDER BY occurred_at DESC,event_id LIMIT $3) e`, who(ctx).WorkspaceID, in.ID, in.Limit))
	})
	huma.Get(api, "/api/v1/events", func(ctx context.Context, in *struct {
		Limit  int       `query:"limit" default:"50" minimum:"1" maximum:"200"`
		Before time.Time `query:"before"`
	}) (*Result[[]map[string]any], error) {
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(e)-'workspace_id'-'payload_hash'),'[]'::jsonb) FROM (SELECT event_id,customer_id,event_name,occurred_at,received_at,status,duplicate_count FROM customer_events WHERE workspace_id=$1 AND ($2::timestamptz='0001-01-01T00:00:00Z' OR received_at<$2) ORDER BY received_at DESC,event_id LIMIT $3) e`, who(ctx).WorkspaceID, in.Before, in.Limit))
	})
	huma.Get(api, "/api/v1/events/{id}", func(ctx context.Context, in *customerID) (*Result[map[string]any], error) {
		return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT to_jsonb(e)-'workspace_id'-'payload_hash' FROM customer_events e WHERE workspace_id=$1 AND event_id=$2`, who(ctx).WorkspaceID, in.ID))
	})

	huma.Post(api, "/api/v1/events", func(ctx context.Context, in *struct{ Body engine.Event }) (*Result[engine.Ingestion], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		out, err := engine.Ingest(ctx, pool, who(ctx).WorkspaceID, in.Body, time.Now().UTC())
		if err != nil {
			return nil, eventError(err)
		}
		return &Result[engine.Ingestion]{Body: out}, nil
	})
	huma.Post(api, "/api/v1/events/batch", func(ctx context.Context, in *struct {
		Body struct {
			Events []engine.Event `json:"events" minItems:"1" maxItems:"100"`
		}
	}) (*Result[[]engine.Ingestion], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		for _, event := range in.Body.Events {
			if err := engine.ValidateEvent(event, now); err != nil {
				return nil, eventError(err)
			}
		}
		results := []engine.Ingestion{}
		for _, event := range in.Body.Events {
			out, err := engine.Ingest(ctx, pool, who(ctx).WorkspaceID, event, now)
			if err != nil {
				message := "Event could not be processed"
				var input *engine.InputError
				if errors.As(err, &input) {
					message = input.Message
				}
				out = engine.Ingestion{EventID: event.EventID, Status: "REJECTED", Executions: []string{}, Error: message}
			}
			results = append(results, out)
		}
		return &Result[[]engine.Ingestion]{Body: results}, nil
	})
	huma.Post(api, "/api/v1/settings/api-keys", func(ctx context.Context, in *struct {
		Body struct {
			Name string `json:"name" minLength:"1" maxLength:"120"`
		}
	}) (*Result[map[string]string], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		token := "cjc_" + randomToken()
		var id string
		err := pool.QueryRow(ctx, `INSERT INTO api_keys(workspace_id,created_by,name,token_hash,prefix) VALUES($1,$2,$3,$4,$5) RETURNING id::text`, who(ctx).WorkspaceID, who(ctx).ID, in.Body.Name, digest(token), token[:12]).Scan(&id)
		if err != nil {
			return nil, dbError(err)
		}
		return &Result[map[string]string]{Body: map[string]string{"id": id, "token": token, "name": in.Body.Name}}, nil
	})
	huma.Get(api, "/api/v1/settings/api-keys", func(ctx context.Context, _ *struct{}) (*Result[[]map[string]any], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(k)),'[]'::jsonb) FROM (SELECT id,name,prefix,revoked_at,created_at FROM api_keys WHERE workspace_id=$1 ORDER BY created_at DESC LIMIT 100) k`, who(ctx).WorkspaceID))
	})
	huma.Delete(api, "/api/v1/settings/api-keys/{id}", func(ctx context.Context, in *journeyID) (*Result[map[string]string], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		var id string
		err := pool.QueryRow(ctx, `UPDATE api_keys SET revoked_at=now() WHERE workspace_id=$1 AND id=$2 RETURNING id::text`, who(ctx).WorkspaceID, in.ID).Scan(&id)
		if err != nil {
			return nil, dbError(err)
		}
		return &Result[map[string]string]{Body: map[string]string{"id": id, "status": "revoked"}}, nil
	})
}
