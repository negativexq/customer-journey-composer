// Package demo provisions an explicitly enabled local-development account.
// Credentials are generated into the ignored environment file, never source code.
package demo

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"net/mail"
	"os"
)

type Credentials struct {
	Enabled  bool   `json:"enabled"`
	Email    string `json:"email,omitempty"`
	Password string `json:"password,omitempty"`
}

func Load(getenv func(string) string) (Credentials, error) {
	if getenv("DEMO_ENABLED") != "true" {
		return Credentials{}, nil
	}
	if getenv("APP_ENV") != "development" {
		return Credentials{}, fmt.Errorf("demo login requires APP_ENV=development")
	}
	c := Credentials{Enabled: true, Email: getenv("DEMO_EMAIL"), Password: getenv("DEMO_PASSWORD")}
	address, err := mail.ParseAddress(c.Email)
	if err != nil || address.Address != c.Email {
		return Credentials{}, fmt.Errorf("valid DEMO_EMAIL is required")
	}
	if len(c.Password) < 16 || len(c.Password) > 72 {
		return Credentials{}, fmt.Errorf("DEMO_PASSWORD must be 16–72 bytes; run make setup")
	}
	return c, nil
}
func Ensure(ctx context.Context, pool *pgxpool.Pool) error {
	c, err := Load(os.Getenv)
	if err != nil || !c.Enabled {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(716294804)`); err != nil {
		return err
	}
	var id, workspace, hash, slug string
	err = tx.QueryRow(ctx, `SELECT u.id::text,u.workspace_id::text,u.password_hash,w.slug FROM users u JOIN workspaces w ON w.id=u.workspace_id WHERE u.email=$1`, c.Email).Scan(&id, &workspace, &hash, &slug)
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == nil && slug != "local-demo" {
		return fmt.Errorf("demo email is already assigned to a different workspace")
	}
	if err == nil && bcrypt.CompareHashAndPassword([]byte(hash), []byte(c.Password)) == nil {
		return tx.Commit(ctx)
	}
	next, err := bcrypt.GenerateFromPassword([]byte(c.Password), 12)
	if err != nil {
		return err
	}
	if id == "" {
		if err = tx.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES('Demo workspace','local-demo') ON CONFLICT(slug) DO UPDATE SET name=workspaces.name RETURNING id::text`).Scan(&workspace); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO users(workspace_id,email,name,password_hash,role) VALUES($1,$2,'Demo User',$3,'Admin')`, workspace, c.Email, string(next)); err != nil {
			return err
		}
	} else {
		if _, err = tx.Exec(ctx, `UPDATE users SET password_hash=$2 WHERE id=$1`, id, string(next)); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `DELETE FROM sessions WHERE user_id=$1`, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
