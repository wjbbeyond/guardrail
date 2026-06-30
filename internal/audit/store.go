package audit

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/wjbbeyond/guardrail/internal/authn"
	"github.com/wjbbeyond/guardrail/internal/store"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Event struct {
	ID               int64     `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	RequestID        string    `json:"request_id"`
	TenantID         string    `json:"tenant_id"`
	Route            string    `json:"route"`
	Provider         string    `json:"provider"`
	Model            string    `json:"model"`
	Status           int       `json:"status"`
	PromptTokens     int       `json:"prompt_tokens"`
	CompletionTokens int       `json:"completion_tokens"`
	CostUSD          float64   `json:"cost_usd"`
	SecurityAction   string    `json:"security_action"`
	LatencyMillis    int64     `json:"latency_ms"`
}

func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Record(ctx context.Context, event Event) error {
	query := `
INSERT INTO audit_events (
  timestamp, request_id, tenant_id, route, provider, model, status,
  prompt_tokens, completion_tokens, cost_usd, security_action, latency_ms
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	tenantID := event.TenantID
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	if _, err := s.db.ExecContext(ctx, query,
		event.Timestamp.UTC().Format(time.RFC3339Nano),
		event.RequestID,
		tenantID,
		event.Route,
		event.Provider,
		event.Model,
		event.Status,
		event.PromptTokens,
		event.CompletionTokens,
		event.CostUSD,
		event.SecurityAction,
		event.LatencyMillis,
	); err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

func (s *Store) Recent(ctx context.Context, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, timestamp, request_id, tenant_id, route, provider, model, status,
  prompt_tokens, completion_tokens, cost_usd, security_action, latency_ms
FROM audit_events
ORDER BY id DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("select audit events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		var timestamp string
		if err := rows.Scan(
			&event.ID,
			&timestamp,
			&event.RequestID,
			&event.TenantID,
			&event.Route,
			&event.Provider,
			&event.Model,
			&event.Status,
			&event.PromptTokens,
			&event.CompletionTokens,
			&event.CostUSD,
			&event.SecurityAction,
			&event.LatencyMillis,
		); err != nil {
			return nil, fmt.Errorf("scan audit event: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return nil, fmt.Errorf("parse audit timestamp: %w", err)
		}
		event.Timestamp = parsed
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit events: %w", err)
	}
	return events, nil
}

func (s *Store) Close() {
	s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	migrator := store.Migrator{
		Namespace: "audit",
		Migrations: []store.Migration{
			{
				Version: 1,
				Name:    "create audit events",
				SQL: `
CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp TEXT NOT NULL,
  request_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL DEFAULT 'default',
  route TEXT NOT NULL,
  provider TEXT NOT NULL,
  model TEXT NOT NULL,
  status INTEGER NOT NULL,
  prompt_tokens INTEGER NOT NULL,
  completion_tokens INTEGER NOT NULL,
  cost_usd REAL NOT NULL,
  security_action TEXT NOT NULL,
  latency_ms INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_events_request_id ON audit_events(request_id);`,
			},
			{
				Version: 2,
				Name:    "audit tenant id",
				Apply:   ensureAuditTenantID,
			},
		},
	}
	return migrator.Run(ctx, s.db)
}

func ensureAuditTenantID(ctx context.Context, db store.SQLRunner) error {
	hasTenantID, err := auditEventsHasTenantID(ctx, db)
	if err != nil {
		return err
	}
	if !hasTenantID {
		if _, err := db.ExecContext(ctx, `ALTER TABLE audit_events ADD COLUMN tenant_id TEXT NOT NULL DEFAULT 'default'`); err != nil {
			return fmt.Errorf("add audit tenant column: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_audit_events_tenant_id ON audit_events(tenant_id)`); err != nil {
		return fmt.Errorf("create audit tenant index: %w", err)
	}
	return nil
}

func auditEventsHasTenantID(ctx context.Context, db store.SQLRunner) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(audit_events)`)
	if err != nil {
		return false, fmt.Errorf("inspect audit events table: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("scan audit column: %w", err)
		}
		if strings.EqualFold(name, "tenant_id") {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate audit columns: %w", err)
	}
	return false, nil
}
