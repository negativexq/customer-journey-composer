// Command admin creates the first production workspace owner using environment
// inputs. Passwords are never flags, logged, or stored in source control.
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/config"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/database"
	"golang.org/x/crypto/bcrypt"
	"net/mail"
	"os"
	"strings"
	"time"
)

func run() error {
	email := strings.ToLower(strings.TrimSpace(os.Getenv("ADMIN_EMAIL")))
	password := os.Getenv("ADMIN_PASSWORD")
	name := os.Getenv("WORKSPACE_NAME")
	a, err := mail.ParseAddress(email)
	if err != nil || a.Address != email || len(password) < 12 || len(password) > 72 || strings.TrimSpace(name) == "" {
		return fmt.Errorf("valid ADMIN_EMAIL, ADMIN_PASSWORD (12–72 bytes) and WORKSPACE_NAME are required")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = database.Ready(ctx, pool); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("password hashing failed")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("database unavailable")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var workspace, user string
	if err = tx.QueryRow(ctx, `INSERT INTO workspaces(name,slug) VALUES($1,$2) RETURNING id::text`, name, "workspace-"+strings.ToLower(rand.Text())).Scan(&workspace); err != nil {
		return fmt.Errorf("workspace creation failed")
	}
	if err = tx.QueryRow(ctx, `INSERT INTO users(workspace_id,email,name,password_hash,role) VALUES($1,$2,'Workspace owner',$3,'Admin') RETURNING id::text`, workspace, email, string(hash)).Scan(&user); err != nil {
		return fmt.Errorf("user creation failed; email may already be assigned")
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_logs(workspace_id,actor_id,action,resource_id) VALUES($1,$2,'workspace.bootstrap',$1)`, workspace, user); err != nil {
		return fmt.Errorf("audit persistence failed")
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("transaction failed")
	}
	fmt.Println("Created production workspace owner.")
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
