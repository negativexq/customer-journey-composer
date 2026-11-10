package httpapi

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"net/mail"
	"strings"
)

func registerUsers(api huma.API, pool *pgxpool.Pool) {
	huma.Get(api, "/api/v1/auth/config", func(_ context.Context, _ *struct{}) (*Result[map[string]any], error) {
		return &Result[map[string]any]{Body: map[string]any{"signup_enabled": signupEnabled()}}, nil
	})
	huma.Get(api, "/api/v1/users", func(ctx context.Context, _ *struct{}) (*Result[[]map[string]any], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		return jsonRow[[]map[string]any](pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(u)),'[]'::jsonb) FROM (SELECT id,name,email,role,created_at FROM users WHERE workspace_id=$1 ORDER BY created_at,id LIMIT 200)u`, who(ctx).WorkspaceID))
	})
	huma.Post(api, "/api/v1/users", func(ctx context.Context, in *struct {
		Body struct {
			Name     string `json:"name" minLength:"1" maxLength:"120"`
			Email    string `json:"email" maxLength:"254"`
			Password string `json:"password" minLength:"12" maxLength:"72"`
			Role     string `json:"role" enum:"Admin,Editor,Viewer"`
		}
	}) (*Result[map[string]any], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		email := strings.ToLower(strings.TrimSpace(in.Body.Email))
		addr, err := mail.ParseAddress(email)
		if err != nil || addr.Address != email {
			return nil, huma.Error422UnprocessableEntity("Valid email required")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(in.Body.Password), 12)
		if err != nil {
			return nil, huma.Error422UnprocessableEntity("Password must be at most 72 bytes")
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		out, err := jsonRow[map[string]any](tx.QueryRow(ctx, `INSERT INTO users(workspace_id,name,email,password_hash,role) VALUES($1,$2,$3,$4,$5) RETURNING jsonb_build_object('id',id,'name',name,'email',email,'role',role)`, who(ctx).WorkspaceID, in.Body.Name, email, string(hash), in.Body.Role))
		if err != nil {
			return nil, err
		}
		id := out.Body["id"].(string)
		if err = audit(ctx, tx, "user.create", &id, map[string]any{"role": in.Body.Role}); err != nil {
			return nil, dbError(err)
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return out, nil
	})
	huma.Put(api, "/api/v1/users/{id}/role", func(ctx context.Context, in *struct {
		ID   string `path:"id" format:"uuid"`
		Body struct {
			Role string `json:"role" enum:"Admin,Editor,Viewer"`
		}
	}) (*Result[map[string]any], error) {
		if who(ctx).Role != "Admin" {
			return nil, huma.Error403Forbidden("Admin role required")
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return nil, dbError(err)
		}
		defer func() { _ = tx.Rollback(context.Background()) }()
		var lock bool
		if err = tx.QueryRow(ctx, `SELECT true FROM workspaces WHERE id=$1 FOR UPDATE`, who(ctx).WorkspaceID).Scan(&lock); err != nil {
			return nil, dbError(err)
		}
		var old string
		if err = tx.QueryRow(ctx, `SELECT role FROM users WHERE workspace_id=$1 AND id=$2 FOR UPDATE`, who(ctx).WorkspaceID, in.ID).Scan(&old); err != nil {
			return nil, dbError(err)
		}
		if old == "Admin" && in.Body.Role != "Admin" {
			var other bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE workspace_id=$1 AND id<>$2 AND role='Admin')`, who(ctx).WorkspaceID, in.ID).Scan(&other); err != nil {
				return nil, dbError(err)
			}
			if !other {
				return nil, huma.Error409Conflict("Workspace must retain at least one Admin")
			}
		}
		out, err := jsonRow[map[string]any](tx.QueryRow(ctx, `UPDATE users SET role=$3 WHERE workspace_id=$1 AND id=$2 RETURNING jsonb_build_object('id',id,'role',role)`, who(ctx).WorkspaceID, in.ID, in.Body.Role))
		if err != nil {
			return nil, err
		}
		if err = audit(ctx, tx, "user.role", &in.ID, map[string]any{"from": old, "to": in.Body.Role}); err != nil {
			return nil, dbError(err)
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, dbError(err)
		}
		return out, nil
	})
}
