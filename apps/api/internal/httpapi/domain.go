package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"os"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/workflow"
	"golang.org/x/crypto/bcrypt"
)

type principalKey struct{}
type Identity struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Workspace   string `json:"workspace"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	CSRF        string `json:"csrf_token"`
}
type Result[T any] struct{ Body T }
type authResponse struct {
	SetCookie string `header:"Set-Cookie"`
	Body      Identity
}
type credentials struct {
	Body struct {
		Email     string `json:"email" maxLength:"254"`
		Password  string `json:"password" minLength:"12" maxLength:"72"`
		Name      string `json:"name,omitempty" maxLength:"120"`
		Workspace string `json:"workspace,omitempty" maxLength:"120"`
	}
}
type Journey struct {
	CreationStep string          `json:"creation_step"`
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Tags         []string        `json:"tags"`
	Status       string          `json:"status"`
	Draft        json.RawMessage `json:"draft"`
	Revision     int             `json:"revision"`
	UpdatedAt    time.Time       `json:"updated_at"`
	Version      *int            `json:"version,omitempty"`
	Stats        *struct {
		Entries   int64 `json:"entries"`
		Active    int64 `json:"active"`
		Completed int64 `json:"completed"`
		Converted int64 `json:"converted"`
	} `json:"stats,omitempty"`
}
type journeyID struct {
	ID string `path:"id" format:"uuid"`
}
type journeyWrite struct {
	Body struct {
		CreationStep string   `json:"creation_step,omitempty" enum:"method,builder"`
		Name         string   `json:"name" minLength:"1" maxLength:"160"`
		Description  string   `json:"description,omitempty" maxLength:"4000"`
		Tags         []string `json:"tags,omitempty" maxItems:"20"`
	}
}
type draftWrite struct {
	ID   string `path:"id" format:"uuid"`
	Body struct {
		Name        string          `json:"name" minLength:"1" maxLength:"160"`
		Description string          `json:"description,omitempty" maxLength:"4000"`
		Tags        []string        `json:"tags,omitempty" maxItems:"20"`
		Draft       json.RawMessage `json:"draft"`
		Revision    int             `json:"revision" minimum:"1"`
	}
}

func randomToken() string              { b := make([]byte, 32); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func digest(s string) string           { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func who(ctx context.Context) Identity { return ctx.Value(principalKey{}).(Identity) }
func canEdit(ctx context.Context) error {
	if who(ctx).Role == "Viewer" {
		return huma.Error403Forbidden("Editor role required")
	}
	return nil
}
func dbError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return huma.Error404NotFound("Record not found")
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return huma.Error409Conflict("Record already exists")
	}
	return huma.Error500InternalServerError("Database operation failed")
}
func jsonRow[T any](row pgx.Row) (*Result[T], error) {
	var b []byte
	if err := row.Scan(&b); err != nil {
		return nil, dbError(err)
	}
	var value T
	if err := json.Unmarshal(b, &value); err != nil {
		return nil, huma.Error500InternalServerError("Invalid stored record")
	}
	return &Result[T]{Body: value}, nil
}
func cookie(token string, expiry time.Time) string {
	return (&http.Cookie{Name: "cjc_session", Value: token, Path: "/", HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") == "true", SameSite: http.SameSiteStrictMode, Expires: expiry, MaxAge: int(time.Until(expiry).Seconds())}).String()
}

func registerDomain(api huma.API, pool *pgxpool.Pool) {
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		if !strings.HasPrefix(ctx.URL().Path, "/api/v1/") {
			next(ctx)
			return
		}
		path := ctx.URL().Path
		if ctx.Method() != "GET" && ctx.Method() != "HEAD" {
			if site := ctx.Header("Sec-Fetch-Site"); site == "cross-site" {
				_ = huma.WriteErr(api, ctx, 403, "Cross-site request rejected")
				return
			}
			if origin := ctx.Header("Origin"); origin != "" {
				parsed := ctx.URL()
				expected := "http://" + ctx.Host()
				if parsed.Scheme == "https" || os.Getenv("COOKIE_SECURE") == "true" {
					expected = "https://" + ctx.Host()
				}
				if origin != expected {
					_ = huma.WriteErr(api, ctx, 403, "Origin does not match this application")
					return
				}
			}
		}
		if path == "/api/v1/auth/register" || path == "/api/v1/auth/login" || path == "/api/v1/auth/demo" || path == "/api/v1/auth/config" {
			next(ctx)
			return
		}

		if (path == "/api/v1/events" || path == "/api/v1/events/batch") && ctx.Method() == "POST" && strings.HasPrefix(ctx.Header("Authorization"), "Bearer ") {
			token := strings.TrimPrefix(ctx.Header("Authorization"), "Bearer ")
			var p Identity
			err := pool.QueryRow(ctx.Context(), `SELECT workspace_id::text,created_by::text FROM api_keys WHERE token_hash=$1 AND revoked_at IS NULL`, digest(token)).Scan(&p.WorkspaceID, &p.ID)
			if err != nil {
				if !errors.Is(err, pgx.ErrNoRows) {
					_ = huma.WriteErr(api, ctx, 503, "Authentication temporarily unavailable")
					return
				}
				_ = huma.WriteErr(api, ctx, 401, "Invalid API key")
				return
			}
			p.Role = "Editor"
			next(huma.WithContext(ctx, context.WithValue(ctx.Context(), principalKey{}, p)))
			return
		}
		req := &http.Request{Header: http.Header{"Cookie": []string{ctx.Header("Cookie")}}}
		c, err := req.Cookie("cjc_session")
		if err != nil {
			_ = huma.WriteErr(api, ctx, 401, "Sign in required")
			return
		}
		var p Identity
		err = pool.QueryRow(ctx.Context(), `SELECT u.id::text,u.workspace_id::text,w.name,u.email,u.name,u.role,s.csrf_token FROM sessions s JOIN users u ON u.id=s.user_id JOIN workspaces w ON w.id=u.workspace_id WHERE s.token_hash=$1 AND s.expires_at>now()`, digest(c.Value)).Scan(&p.ID, &p.WorkspaceID, &p.Workspace, &p.Email, &p.Name, &p.Role, &p.CSRF)
		if err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				_ = huma.WriteErr(api, ctx, 503, "Authentication temporarily unavailable")
				return
			}
			_ = huma.WriteErr(api, ctx, 401, "Session expired or invalid")
			return
		}
		if ctx.Method() != "GET" && ctx.Method() != "HEAD" && ctx.Header("X-CSRF-Token") != p.CSRF {
			_ = huma.WriteErr(api, ctx, 403, "CSRF token is required")
			return
		}
		next(huma.WithContext(ctx, context.WithValue(ctx.Context(), principalKey{}, p)))
	})
	registerSecurityMiddleware(api)
	authSlots := make(chan struct{}, 8)
	auth := func(register bool) func(context.Context, *credentials) (*authResponse, error) {
		return func(ctx context.Context, in *credentials) (*authResponse, error) {
			select {
			case authSlots <- struct{}{}:
				defer func() { <-authSlots }()
			default:
				return nil, huma.Error429TooManyRequests("Authentication is busy; retry shortly")
			}
			if register && !signupEnabled() {
				return nil, huma.Error403Forbidden("Public workspace registration is disabled")
			}
			email := strings.ToLower(strings.TrimSpace(in.Body.Email))
			addr, err := mail.ParseAddress(email)
			if err != nil || addr.Address != email {
				return nil, huma.Error422UnprocessableEntity("Valid email required")
			}
			tx, err := pool.Begin(ctx)
			if err != nil {
				return nil, dbError(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			var p Identity
			if register {
				if strings.TrimSpace(in.Body.Name) == "" || strings.TrimSpace(in.Body.Workspace) == "" {
					return nil, huma.Error422UnprocessableEntity("Name and workspace are required")
				}
				hash, err := bcrypt.GenerateFromPassword([]byte(in.Body.Password), 12)
				if err != nil {
					return nil, huma.Error422UnprocessableEntity("Invalid password")
				}
				err = tx.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES($1,$2) RETURNING id::text`, strings.TrimSpace(in.Body.Workspace), "workspace-"+randomToken()[:16]).Scan(&p.WorkspaceID)
				if err != nil {
					return nil, dbError(err)
				}
				err = tx.QueryRow(ctx, `INSERT INTO users(workspace_id,email,name,password_hash) VALUES($1,$2,$3,$4) RETURNING id::text,role`, p.WorkspaceID, email, strings.TrimSpace(in.Body.Name), string(hash)).Scan(&p.ID, &p.Role)
				if err != nil {
					return nil, dbError(err)
				}
				p.Email = email
				p.Name = strings.TrimSpace(in.Body.Name)
				p.Workspace = strings.TrimSpace(in.Body.Workspace)
			} else {
				var hash string
				err = tx.QueryRow(ctx, `SELECT u.id::text,u.workspace_id::text,w.name,u.email,u.name,u.role,u.password_hash FROM users u JOIN workspaces w ON w.id=u.workspace_id WHERE u.email=$1`, email).Scan(&p.ID, &p.WorkspaceID, &p.Workspace, &p.Email, &p.Name, &p.Role, &hash)
				if err != nil {
					hash = string(dummyPasswordHash)
				}
				valid := bcrypt.CompareHashAndPassword([]byte(hash), []byte(in.Body.Password)) == nil
				if err != nil || !valid {
					return nil, huma.Error401Unauthorized("Invalid email or password")
				}
			}
			token := randomToken()
			p.CSRF = randomToken()
			expiry := time.Now().Add(24 * time.Hour)
			if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE expires_at<now()`); err != nil {
				return nil, dbError(err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at) VALUES($1,$2,$3,$4)`, digest(token), p.ID, p.CSRF, expiry); err != nil {
				return nil, dbError(err)
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, dbError(err)
			}
			return &authResponse{SetCookie: cookie(token, expiry), Body: p}, nil
		}
	}
	huma.Post(api, "/api/v1/auth/register", auth(true))
	huma.Post(api, "/api/v1/auth/login", auth(false))
	huma.Get(api, "/api/v1/auth/me", func(ctx context.Context, _ *struct{}) (*Result[Identity], error) {
		return &Result[Identity]{Body: who(ctx)}, nil
	})
	huma.Post(api, "/api/v1/auth/logout", func(ctx context.Context, in *struct {
		Cookie string `header:"Cookie"`
	}) (*struct {
		SetCookie string `header:"Set-Cookie"`
		Body      struct {
			Status string `json:"status"`
		}
	}, error) {
		req := &http.Request{Header: http.Header{"Cookie": []string{in.Cookie}}}
		c, _ := req.Cookie("cjc_session")
		if c != nil {
			if _, err := pool.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", digest(c.Value)); err != nil {
				return nil, dbError(err)
			}
		}
		out := &struct {
			SetCookie string `header:"Set-Cookie"`
			Body      struct {
				Status string `json:"status"`
			}
		}{SetCookie: (&http.Cookie{Name: "cjc_session", Value: "", Path: "/", HttpOnly: true, Secure: os.Getenv("COOKIE_SECURE") == "true", SameSite: http.SameSiteStrictMode, MaxAge: -1}).String()}
		out.Body.Status = "signed_out"
		return out, nil
	})
	huma.Post(api, "/api/v1/journeys", func(ctx context.Context, in *journeyWrite) (*Result[Journey], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.Body.Name) == "" {
			return nil, huma.Error422UnprocessableEntity("Journey name required")
		}
		if in.Body.Tags == nil {
			in.Body.Tags = []string{}
		}
		return jsonRow[Journey](pool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,description,tags,creation_step) VALUES($1,$2,$3,$4,COALESCE(NULLIF($5,''),'builder')) RETURNING to_jsonb(journeys)-'workspace_id'`, who(ctx).WorkspaceID, strings.TrimSpace(in.Body.Name), in.Body.Description, in.Body.Tags, in.Body.CreationStep))
	})
	huma.Get(api, "/api/v1/journeys", func(ctx context.Context, in *journeyListInput) (*Result[[]Journey], error) {
		return listJourneys(ctx, pool, in)
	})
	huma.Get(api, "/api/v1/journeys/{id}", func(ctx context.Context, in *journeyID) (*Result[Journey], error) {
		return jsonRow[Journey](pool.QueryRow(ctx, `SELECT (to_jsonb(j)-'workspace_id')||jsonb_build_object('version',v.version) FROM journeys j LEFT JOIN journey_versions v ON v.id=j.current_version_id WHERE j.workspace_id=$1 AND j.id=$2`, who(ctx).WorkspaceID, in.ID))
	})
	huma.Put(api, "/api/v1/journeys/{id}/draft", func(ctx context.Context, in *draftWrite) (*Result[Journey], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		if len(in.Body.Draft) > 1<<20 || !json.Valid(in.Body.Draft) || strings.TrimSpace(in.Body.Name) == "" {
			return nil, huma.Error422UnprocessableEntity("Valid draft and name required; maximum draft size is 1 MiB")
		}
		if _, err := workflow.DecodeDraft(in.Body.Draft); err != nil {
			return nil, huma.Error422UnprocessableEntity(err.Error())
		}
		if in.Body.Tags == nil {
			in.Body.Tags = []string{}
		}
		result, err := jsonRow[Journey](pool.QueryRow(ctx, `UPDATE journeys SET name=$3,description=$4,tags=$5,draft=$6,revision=revision+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND revision=$7 AND status<>'Archived' AND creation_step='builder' RETURNING to_jsonb(journeys)-'workspace_id'`, who(ctx).WorkspaceID, in.ID, strings.TrimSpace(in.Body.Name), in.Body.Description, in.Body.Tags, in.Body.Draft, in.Body.Revision))
		if err != nil {
			var exists bool
			_ = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM journeys WHERE workspace_id=$1 AND id=$2)", who(ctx).WorkspaceID, in.ID).Scan(&exists)
			if exists {
				return nil, huma.Error409Conflict("Draft changed or journey archived; reload before saving")
			}
		}
		return result, err
	})
	huma.Post(api, "/api/v1/journeys/{id}/duplicate", func(ctx context.Context, in *journeyID) (*Result[Journey], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		return jsonRow[Journey](pool.QueryRow(ctx, `INSERT INTO journeys(workspace_id,name,description,tags,draft) SELECT workspace_id,left(name,150)||' (copy)',description,tags,draft FROM journeys WHERE workspace_id=$1 AND id=$2 RETURNING to_jsonb(journeys)-'workspace_id'`, who(ctx).WorkspaceID, in.ID))
	})
	huma.Delete(api, "/api/v1/journeys/{id}", func(ctx context.Context, in *journeyID) (*Result[Journey], error) {
		if err := canEdit(ctx); err != nil {
			return nil, err
		}
		return jsonRow[Journey](pool.QueryRow(ctx, `UPDATE journeys SET status='Archived',revision=revision+1,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND status='Draft' RETURNING to_jsonb(journeys)-'workspace_id'`, who(ctx).WorkspaceID, in.ID))
	})
	registerWorkflows(api, pool)
	registerCustomers(api, pool)
	registerExecutions(api, pool)
	registerMessages(api, pool)
	registerDataCatalog(api, pool)
	registerComposition(api, pool)
	registerAI(api, pool)
	registerTesting(api, pool)
	registerInbox(api, pool)
	registerAnalytics(api, pool)
	registerInsights(api, pool)
	registerAudience(api, pool)
	registerDemo(api)
	registerLifecycle(api, pool)
	registerExecutionControls(api, pool)
	registerPublishChecks(api, pool)
	registerUsers(api, pool)
	registerSystem(api, pool)
	registerTemplates(api, pool)
}

var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte(randomToken()), 12)
