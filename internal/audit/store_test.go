package audit

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/wjbbeyond/guardrail/internal/authn"

	_ "modernc.org/sqlite"
)

func TestOpen_migratesLegacyAuditEventsToDefaultTenant(t *testing.T) {
	// Given
	ctx := context.Background()
	dsn := "file:" + filepath.Join(t.TempDir(), "audit.db") + "?_pragma=busy_timeout(5000)"
	seedLegacyAuditEvents(t, ctx, dsn)

	// When
	store, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open() error = %v, want nil", err)
	}
	defer store.Close()
	events, err := store.Recent(ctx, 10)
	if err != nil {
		t.Fatalf("Recent() error = %v, want nil", err)
	}

	// Then
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].TenantID != authn.DefaultTenantID {
		t.Fatalf("tenant = %s, want %s", events[0].TenantID, authn.DefaultTenantID)
	}
}

func seedLegacyAuditEvents(t *testing.T, ctx context.Context, dsn string) {
	t.Helper()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open legacy sqlite: %v", err)
	}
	defer db.Close()
	queries := []string{
		`CREATE TABLE audit_events (
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
)`,
		`INSERT INTO audit_events (
  timestamp, request_id, route, provider, model, status,
  prompt_tokens, completion_tokens, cost_usd, security_action, latency_ms
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	}
	if _, err := db.ExecContext(ctx, queries[0]); err != nil {
		t.Fatalf("create legacy audit_events: %v", err)
	}
	if _, err := db.ExecContext(ctx, queries[1],
		time.Now().UTC().Format(time.RFC3339Nano),
		"req-1",
		"/v1/chat/completions",
		"mock",
		"gpt-4o-mini",
		200,
		10,
		3,
		0.00001,
		"allow",
		12,
	); err != nil {
		t.Fatalf("insert legacy audit event: %v", err)
	}
}
