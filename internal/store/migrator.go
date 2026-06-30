package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Migration struct {
	Version int
	Name    string
	SQL     string
	Apply   func(ctx context.Context, db SQLRunner) error
}

type SQLRunner interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type Migrator struct {
	Namespace  string
	Migrations []Migration
}

func (m Migrator) Run(ctx context.Context, db *sql.DB) error {
	if err := ensureMigrationTable(ctx, db); err != nil {
		return err
	}
	applied, err := appliedVersions(ctx, db, m.Namespace)
	if err != nil {
		return err
	}
	for _, migration := range m.Migrations {
		if applied[migration.Version] {
			continue
		}
		if err := runMigration(ctx, db, m.Namespace, migration); err != nil {
			return err
		}
	}
	return nil
}

func ensureMigrationTable(ctx context.Context, db *sql.DB) error {
	query := `
CREATE TABLE IF NOT EXISTS schema_migrations (
  namespace TEXT NOT NULL,
  version INTEGER NOT NULL,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL,
  PRIMARY KEY (namespace, version)
);`
	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}
	return nil
}

func appliedVersions(ctx context.Context, db *sql.DB, namespace string) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations WHERE namespace = ?`, namespace)
	if err != nil {
		return nil, fmt.Errorf("select schema migrations: %w", err)
	}
	defer rows.Close()
	applied := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan schema migration: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate schema migrations: %w", err)
	}
	return applied, nil
}

func runMigration(ctx context.Context, db *sql.DB, namespace string, migration Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s %d: %w", namespace, migration.Version, err)
	}
	if err := applyMigration(ctx, tx, migration); err != nil {
		tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO schema_migrations (namespace, version, name, applied_at)
VALUES (?, ?, ?, ?)`, namespace, migration.Version, migration.Name, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		tx.Rollback()
		return fmt.Errorf("record migration %s %d: %w", namespace, migration.Version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s %d: %w", namespace, migration.Version, err)
	}
	return nil
}

func applyMigration(ctx context.Context, tx *sql.Tx, migration Migration) error {
	if migration.Apply != nil {
		if err := migration.Apply(ctx, txRunner{tx: tx}); err != nil {
			return fmt.Errorf("apply migration %d %s: %w", migration.Version, migration.Name, err)
		}
		return nil
	}
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return fmt.Errorf("apply migration %d %s: %w", migration.Version, migration.Name, err)
	}
	return nil
}

type txRunner struct {
	tx *sql.Tx
}

func (db txRunner) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return db.tx.ExecContext(ctx, query, args...)
}

func (db txRunner) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return db.tx.QueryContext(ctx, query, args...)
}
