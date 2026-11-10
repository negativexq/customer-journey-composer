package database

import (
	"context"
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/negativexq/customer-journey-composer/apps/api/internal/observability"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// Do not echo the DSN, which may contain credentials.
		return nil, fmt.Errorf("invalid database connection configuration")
	}
	cfg.MaxConns = 20
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.ConnectTimeout = 5 * time.Second
	cfg.ConnConfig.Tracer = observability.PGXTracer{}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database is unavailable")
	}
	return pool, nil
}

// Migrate serializes deployers using a transaction-scoped advisory lock. A failed
// migration rolls back the entire batch; changed applied files are rejected.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(1128811075)"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version text PRIMARY KEY,
		checksum text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	slices.Sort(files)
	for _, file := range files {
		contents, err := migrations.ReadFile(file)
		if err != nil {
			return err
		}
		checksum := fmt.Sprintf("%x", sha256.Sum256(contents))
		var appliedChecksum string
		err = tx.QueryRow(ctx, "SELECT checksum FROM schema_migrations WHERE version = $1", file).Scan(&appliedChecksum)
		if err == nil {
			if appliedChecksum != checksum {
				return fmt.Errorf("applied migration %s has changed; add a new migration instead", file)
			}
			continue
		}
		if err != pgx.ErrNoRows {
			return err
		}
		if _, err := tx.Exec(ctx, string(contents)); err != nil {
			return fmt.Errorf("apply %s: %w", file, err)
		}
		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, checksum) VALUES ($1, $2)", file, checksum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Ready checks connectivity and migration compatibility. The API never silently
// starts serving an outdated schema; migrations run separately with deploy access.
func Ready(ctx context.Context, pool *pgxpool.Pool) error {
	files, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	rows, err := pool.Query(ctx, "SELECT version,checksum FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("schema is not ready")
	}
	applied := map[string]string{}
	for rows.Next() {
		var name, checksum string
		if err := rows.Scan(&name, &checksum); err != nil {
			rows.Close()
			return fmt.Errorf("schema is not ready")
		}
		applied[name] = checksum
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fmt.Errorf("schema is not ready")
	}
	if len(applied) != len(files) {
		return fmt.Errorf("schema version does not match this API")
	}
	for _, file := range files {
		contents, err := migrations.ReadFile(file)
		if err != nil {
			return err
		}
		checksum, ok := applied[file]
		if !ok {
			return fmt.Errorf("schema migration is missing")
		}
		if checksum != fmt.Sprintf("%x", sha256.Sum256(contents)) {
			return fmt.Errorf("schema checksum mismatch")
		}
	}
	return nil
}
