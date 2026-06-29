package audit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Event struct {
	ID               int64     `json:"id"`
	Timestamp        time.Time `json:"timestamp"`
	RequestID        string    `json:"request_id"`
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
  timestamp, request_id, route, provider, model, status,
  prompt_tokens, completion_tokens, cost_usd, security_action, latency_ms
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := s.db.ExecContext(ctx, query,
		event.Timestamp.UTC().Format(time.RFC3339Nano),
		event.RequestID,
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
SELECT id, timestamp, request_id, route, provider, model, status,
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
	query := `
CREATE TABLE IF NOT EXISTS audit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp TEXT NOT NULL,
  request_id TEXT NOT NULL,
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
CREATE INDEX IF NOT EXISTS idx_audit_events_request_id ON audit_events(request_id);`
	if _, err := s.db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("migrate audit store: %w", err)
	}
	return nil
}
