package httpapi

import (
	"context"
	"encoding/json"
	"net/mail"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/delivery"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/messaging"
	engine "github.com/negativexq/customer-journey-composer/apps/api/internal/runtime"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
)

type previewInput struct {
	Channel    string          `json:"channel" enum:"email,sms"`
	Config     workflow.Config `json:"config"`
	CustomerID string          `json:"customer_id,omitempty" maxLength:"128"`
	Customer   map[string]any  `json:"customer,omitempty"`
	Event      map[string]any  `json:"event,omitempty"`
}
type messagePolicy struct {
	FrequencyLimit         int      `json:"frequency_limit" minimum:"1" maximum:"1000"`
	FrequencyWindowSeconds int      `json:"frequency_window_seconds" minimum:"60" maximum:"2592000"`
	TestAllowlist          []string `json:"test_allowlist" maxItems:"100"`
}

func previewMessage(ctx context.Context, pool *pgxpool.Pool, input previewInput) (messaging.Message, error) {
	customer := map[string]any{"locale": "en", "timezone": "UTC", "attributes": map[string]any{}, "consents": map[string]any{"email": false, "sms": false}}
	if input.CustomerID != "" {
		var raw []byte
		if err := pool.QueryRow(ctx, `SELECT to_jsonb(c)-'workspace_id' FROM customers c WHERE workspace_id=$1 AND customer_id=$2`, who(ctx).WorkspaceID, input.CustomerID).Scan(&raw); err != nil {
			return messaging.Message{}, dbError(err)
		}
		if err := json.Unmarshal(raw, &customer); err != nil {
			return messaging.Message{}, huma.Error500InternalServerError("Invalid customer")
		}
	}
	for key, value := range input.Customer {
		if key == "attributes" {
			existing, _ := customer[key].(map[string]any)
			if existing == nil {
				existing = map[string]any{}
			}
			if override, ok := value.(map[string]any); ok {
				for k, v := range override {
					existing[k] = v
				}
				customer[key] = existing
			} else {
				return messaging.Message{}, huma.Error422UnprocessableEntity("Attributes must be an object")
			}
		} else {
			customer[key] = value
		}
	}
	message, err := messaging.Render(input.Channel, input.Config, engine.ContextData(customer, input.Event, map[string]any{}))
	if err != nil {
		return message, huma.Error422UnprocessableEntity(err.Error())
	}
	return message, nil
}
func registerMessages(api huma.API, pool *pgxpool.Pool) {
	huma.Post(api, "/api/v1/messages/preview", func(ctx context.Context, in *struct{ Body previewInput }) (*Result[messaging.Message], error) {
		message, err := previewMessage(ctx, pool, in.Body)
		if err != nil {
			return nil, err
		}
		return &Result[messaging.Message]{Body: message}, nil
	})
	huma.Get(api, "/api/v1/settings/messaging", func(ctx context.Context, _ *struct{}) (*Result[messagePolicy], error) {
		out := messagePolicy{FrequencyLimit: 3, FrequencyWindowSeconds: 86400, TestAllowlist: []string{}}
		var raw []byte
		err := pool.QueryRow(ctx, `SELECT frequency_limit,frequency_window_seconds,test_allowlist FROM workspace_settings WHERE workspace_id=$1`, who(ctx).WorkspaceID).Scan(&out.FrequencyLimit, &out.FrequencyWindowSeconds, &raw)
		if err == nil {
			_ = json.Unmarshal(raw, &out.TestAllowlist)
		} else if err != pgx.ErrNoRows {
			return nil, dbError(err)
		}
		return &Result[messagePolicy]{Body: out}, nil
	})
	huma.Put(api, "/api/v1/settings/messaging", func(ctx context.Context, in *struct{ Body messagePolicy }) (*Result[messagePolicy], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		in.Body.TestAllowlist = delivery.NormalizeAllowlist(in.Body.TestAllowlist)
		for _, address := range in.Body.TestAllowlist {
			if strings.HasPrefix(address, "+") {
				if len(address) < 8 || len(address) > 16 {
					return nil, huma.Error422UnprocessableEntity("Invalid test phone")
				}
			} else if value, err := mail.ParseAddress(address); err != nil || value.Address != address {
				return nil, huma.Error422UnprocessableEntity("Invalid test email")
			}
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		if _, err = tx.Exec(ctx, `INSERT INTO workspace_settings(workspace_id,frequency_limit,frequency_window_seconds,test_allowlist) VALUES($1,$2,$3,$4) ON CONFLICT(workspace_id) DO UPDATE SET frequency_limit=$2,frequency_window_seconds=$3,test_allowlist=$4,updated_at=now()`, who(ctx).WorkspaceID, in.Body.FrequencyLimit, in.Body.FrequencyWindowSeconds, in.Body.TestAllowlist); err != nil {
			return nil, dbError(err)
		}
		workspace := who(ctx).WorkspaceID
		if err = audit(ctx, tx, "workspace.messaging_policy", &workspace, map[string]any{"frequency_limit": in.Body.FrequencyLimit, "frequency_window_seconds": in.Body.FrequencyWindowSeconds, "allowlist_size": len(in.Body.TestAllowlist)}); err != nil {
			return nil, dbError(err)
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}

		return &Result[messagePolicy]{Body: in.Body}, nil
	})
	huma.Post(api, "/api/v1/messages/test-send", func(ctx context.Context, in *struct {
		Key  string `header:"Idempotency-Key" minLength:"16" maxLength:"128" required:"true"`
		Body struct {
			Preview   previewInput `json:"preview"`
			Recipient string       `json:"recipient" minLength:"1" maxLength:"254"`
		}
	}) (*Result[map[string]any], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		message, err := previewMessage(ctx, pool, in.Body.Preview)
		if err != nil {
			return nil, err
		}
		if len(message.Missing) > 0 {
			return nil, huma.Error422UnprocessableEntity("Resolve missing personalization before test sending")
		}
		recipient := strings.ToLower(strings.TrimSpace(in.Body.Recipient))
		var raw []byte
		if err := pool.QueryRow(ctx, `SELECT test_allowlist FROM workspace_settings WHERE workspace_id=$1`, who(ctx).WorkspaceID).Scan(&raw); err != nil {
			return nil, huma.Error403Forbidden("Configure the test recipient allowlist first")
		}
		var list []string
		_ = json.Unmarshal(raw, &list)
		allowed := false
		for _, value := range list {
			allowed = allowed || strings.EqualFold(value, recipient)
		}
		if !allowed {
			return nil, huma.Error403Forbidden("Test recipient is not allowlisted")
		}
		message.To = recipient
		body, _ := json.Marshal(message)
		fingerprint := digest(string(body))
		provider := "smtp"
		idempotent := false
		if message.Channel == "sms" {
			provider = "mock_sms"
			idempotent = true
		}
		now := time.Now().UTC()
		_, err = pool.Exec(ctx, `INSERT INTO delivery_jobs(workspace_id,mode,channel,provider,idempotent_provider,logical_key,request_hash,message,decision,status,due_at,created_at) VALUES($1,'TEST',$2,$3,$4,$5,$6,$7,'{"eligible":true,"reason":"allowlisted_test_recipient"}','SCHEDULED',$8,$8) ON CONFLICT(workspace_id,logical_key) DO NOTHING`, who(ctx).WorkspaceID, message.Channel, provider, idempotent, "test:"+in.Key, fingerprint, body, now)
		if err != nil {
			return nil, dbError(err)
		}
		var existing string
		if err := pool.QueryRow(ctx, `SELECT request_hash FROM delivery_jobs WHERE workspace_id=$1 AND logical_key=$2`, who(ctx).WorkspaceID, "test:"+in.Key).Scan(&existing); err != nil {
			return nil, dbError(err)
		}
		if existing != fingerprint {
			return nil, huma.Error409Conflict("Idempotency key was used for a different test message")
		}
		return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT jsonb_build_object('id',id,'status',status,'mode',mode) FROM delivery_jobs WHERE workspace_id=$1 AND logical_key=$2`, who(ctx).WorkspaceID, "test:"+in.Key))
	})
	huma.Get(api, "/api/v1/deliveries", func(ctx context.Context, in *struct {
		CustomerID string `query:"customer_id" maxLength:"128"`
		Limit      int    `query:"limit" default:"50" minimum:"1" maximum:"200"`
	}) (*Result[[]map[string]any], error) {
		if in.CustomerID != "" {
			return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(d)),'[]'::jsonb) FROM (SELECT id,execution_id,node_id,customer_id,channel,provider,mode,status,attempts,decision,created_at,accepted_at,cancel_requested FROM delivery_jobs WHERE workspace_id=$1 AND customer_id=$2 ORDER BY created_at DESC,id LIMIT $3) d`, who(ctx).WorkspaceID, in.CustomerID, in.Limit))
		}
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(d)),'[]'::jsonb) FROM (SELECT id,execution_id,node_id,customer_id,channel,provider,mode,status,attempts,decision,created_at,accepted_at,cancel_requested FROM delivery_jobs WHERE workspace_id=$1 ORDER BY created_at DESC,id LIMIT $2) d`, who(ctx).WorkspaceID, in.Limit))
	})
	huma.Get(api, "/api/v1/deliveries/{id}", func(ctx context.Context, in *journeyID) (*Result[map[string]any], error) {
		return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT (to_jsonb(d)-'workspace_id'-'claim_token'-'request_hash')||jsonb_build_object('attempt_history',COALESCE((SELECT jsonb_agg(to_jsonb(a)-'workspace_id' ORDER BY attempt) FROM delivery_attempts a WHERE a.delivery_id=d.id),'[]'::jsonb)) FROM delivery_jobs d WHERE d.workspace_id=$1 AND d.id=$2`, who(ctx).WorkspaceID, in.ID))
	})
	huma.Get(api, "/api/v1/message-templates", func(ctx context.Context, _ *struct{}) (*Result[[]map[string]any], error) {
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(t)),'[]'::jsonb) FROM (SELECT id,name,channel,updated_at FROM message_templates WHERE workspace_id=$1 ORDER BY updated_at DESC,id LIMIT 100) t`, who(ctx).WorkspaceID))
	})
	huma.Get(api, "/api/v1/message-templates/{id}", func(ctx context.Context, in *journeyID) (*Result[map[string]any], error) {
		return jsonRow[map[string]any](pool.QueryRow(ctx, `SELECT to_jsonb(t)-'workspace_id' FROM message_templates t WHERE workspace_id=$1 AND id=$2`, who(ctx).WorkspaceID, in.ID))
	})

	huma.Post(api, "/api/v1/message-templates", func(ctx context.Context, in *struct {
		Body struct {
			Name    string          `json:"name" minLength:"1" maxLength:"160"`
			Channel string          `json:"channel" enum:"email,sms"`
			Config  workflow.Config `json:"config"`
		}
	}) (*Result[map[string]any], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		if in.Body.Channel == "email" && (strings.TrimSpace(in.Body.Config.Subject) == "" || in.Body.Config.HTML == "" && in.Body.Config.Text == "") || in.Body.Channel == "sms" && in.Body.Config.Text == "" {
			return nil, huma.Error422UnprocessableEntity("Message content required")
		}
		return jsonRow[map[string]any](pool.QueryRow(ctx, `INSERT INTO message_templates(workspace_id,name,channel,config) VALUES($1,$2,$3,$4) RETURNING to_jsonb(message_templates)-'workspace_id'`, who(ctx).WorkspaceID, in.Body.Name, in.Body.Channel, in.Body.Config))
	})
}
