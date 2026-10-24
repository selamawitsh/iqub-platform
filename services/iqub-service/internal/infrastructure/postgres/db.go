package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Open connects to PostgreSQL, checks the connection works, and applies any
// migrations that have not run yet. databaseURL looks like
// postgres://user:password@host:5432/dbname?sslmode=disable
func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("DATABASE_URL is required")
	}

	config, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		// Do not wrap the parse error: it can echo the URL, which holds the password.
		return nil, errors.New("DATABASE_URL is not a valid connection string")
	}

	db := stdlib.OpenDB(*config)
	db.SetMaxOpenConns(15)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run database migrations: %w", err)
	}
	return db, nil
}

// migrate applies every file in migrations/ that is not yet recorded in the
// schema_migrations table, in file name order. Running it again is safe.
func migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}

	names, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	for _, name := range names {
		if err := applyMigration(ctx, db, name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, name string) error {
	script, err := migrationFiles.ReadFile(name)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // does nothing after a successful commit

	// Only one instance migrates at a time, even if two start together.
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(727274)`); err != nil {
		return err
	}

	// Check inside the lock, so an instance that waited sees the other's work.
	var applied bool
	err = tx.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name,
	).Scan(&applied)
	if err != nil {
		return err
	}
	if applied {
		return nil
	}

	if _, err := tx.ExecContext(ctx, string(script)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(name) VALUES ($1)`, name); err != nil {
		return err
	}
	return tx.Commit()
}
