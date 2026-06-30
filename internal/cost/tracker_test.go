package cost

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/wjbbeyond/guardrail/internal/config"
)

type fakeClock struct {
	now time.Time
}

func (f fakeClock) Now() time.Time {
	return f.now
}

func TestTracker_Allow_rejectsRequest_whenDailyBudgetWouldBeExceeded(t *testing.T) {
	// Given
	ctx := context.Background()
	tracker := NewTracker(config.CostConfig{DailyBudgetUSD: 0.000001, PerRequestBudgetUSD: 1}, fakeClock{now: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)})
	if _, err := tracker.Record(ctx, "gpt-4o", 1000, 1000); err != nil {
		t.Fatalf("record usage: %v", err)
	}

	// When
	decision, err := tracker.Allow(ctx, "gpt-4o", 1000, 1000)

	// Then
	if err != nil {
		t.Fatalf("Allow() error = %v, want nil", err)
	}
	if decision.Allowed {
		t.Fatal("expected request to be rejected")
	}
	if decision.Reason != "daily_budget_exceeded" {
		t.Fatalf("reason = %s, want daily_budget_exceeded", decision.Reason)
	}
}

func TestTracker_Snapshot_persistsSpendAcrossTrackers_whenUsingSQLiteLedger(t *testing.T) {
	// Given
	ctx := context.Background()
	dsn := "file:" + t.TempDir() + "/costs.db?_pragma=busy_timeout(5000)"
	now := fakeClock{now: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	cfg := config.CostConfig{DailyBudgetUSD: 1, PerRequestBudgetUSD: 1}
	firstLedger, err := OpenSQLiteLedger(ctx, dsn)
	if err != nil {
		t.Fatalf("open first ledger: %v", err)
	}
	defer firstLedger.Close()
	first := NewTrackerWithLedger(cfg, now, firstLedger)
	usage, err := first.Record(ctx, "gpt-4o-mini", 1000, 1000)
	if err != nil {
		t.Fatalf("record usage: %v", err)
	}
	secondLedger, err := OpenSQLiteLedger(ctx, dsn)
	if err != nil {
		t.Fatalf("open second ledger: %v", err)
	}
	defer secondLedger.Close()
	second := NewTrackerWithLedger(cfg, now, secondLedger)

	// When
	snapshot, err := second.Snapshot(ctx)

	// Then
	if err != nil {
		t.Fatalf("Snapshot() error = %v, want nil", err)
	}
	if snapshot.SpentUSD != usage.CostUSD {
		t.Fatalf("spent = %.8f, want %.8f", snapshot.SpentUSD, usage.CostUSD)
	}
}

func TestTracker_AllowTenant_usesTenantBudgetOverride(t *testing.T) {
	// Given
	ctx := context.Background()
	tracker := NewTrackerWithOptions(TrackerOptions{
		Cost:  config.CostConfig{DailyBudgetUSD: 1, PerRequestBudgetUSD: 1},
		Clock: fakeClock{now: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)},
		Tenants: []config.TenantConfig{{
			ID:                  "limited",
			DailyBudgetUSD:      1,
			PerRequestBudgetUSD: 0.000001,
		}},
	})

	// When
	decision, err := tracker.AllowTenant(ctx, "limited", "gpt-4o", 1000, 1000)

	// Then
	if err != nil {
		t.Fatalf("AllowTenant() error = %v, want nil", err)
	}
	if decision.Allowed {
		t.Fatal("expected tenant request to be rejected")
	}
	if decision.Reason != "per_request_budget_exceeded" {
		t.Fatalf("reason = %s, want per_request_budget_exceeded", decision.Reason)
	}
}

func TestTracker_SnapshotTenant_isolatesSpendByTenant_whenUsingSQLiteLedger(t *testing.T) {
	// Given
	ctx := context.Background()
	dsn := "file:" + t.TempDir() + "/tenant-costs.db?_pragma=busy_timeout(5000)"
	ledger, err := OpenSQLiteLedger(ctx, dsn)
	if err != nil {
		t.Fatalf("open ledger: %v", err)
	}
	defer ledger.Close()
	tracker := NewTrackerWithOptions(TrackerOptions{
		Cost:   config.CostConfig{DailyBudgetUSD: 1, PerRequestBudgetUSD: 1},
		Clock:  fakeClock{now: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)},
		Ledger: ledger,
	})
	if _, err := tracker.RecordTenant(ctx, "acme", "gpt-4o-mini", 1000, 1000); err != nil {
		t.Fatalf("record acme usage: %v", err)
	}

	// When
	acme, err := tracker.SnapshotTenant(ctx, "acme")
	if err != nil {
		t.Fatalf("snapshot acme: %v", err)
	}
	other, err := tracker.SnapshotTenant(ctx, "other")
	if err != nil {
		t.Fatalf("snapshot other: %v", err)
	}

	// Then
	if acme.SpentUSD == 0 {
		t.Fatal("acme spent = 0, want recorded spend")
	}
	if other.SpentUSD != 0 {
		t.Fatalf("other spent = %.8f, want 0", other.SpentUSD)
	}
}

func TestSQLiteLedger_Open_migratesLegacySpendToDefaultTenant(t *testing.T) {
	// Given
	ctx := context.Background()
	dsn := "file:" + t.TempDir() + "/legacy-costs.db?_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
CREATE TABLE cost_spend (
  day TEXT PRIMARY KEY,
  spent_usd REAL NOT NULL,
  updated_at TEXT NOT NULL
);
INSERT INTO cost_spend (day, spent_usd, updated_at)
VALUES ('2026-06-30', 0.42, '2026-06-30T00:00:00Z');`); err != nil {
		t.Fatalf("seed legacy db: %v", err)
	}
	db.Close()

	// When
	ledger, err := OpenSQLiteLedger(ctx, dsn)
	if err != nil {
		t.Fatalf("open migrated ledger: %v", err)
	}
	defer ledger.Close()
	spent, err := ledger.Spend(ctx, "default", "2026-06-30")

	// Then
	if err != nil {
		t.Fatalf("Spend() error = %v, want nil", err)
	}
	if spent != 0.42 {
		t.Fatalf("spent = %.2f, want 0.42", spent)
	}
}

func TestEstimateTokens_roundsUpByFourRunes(t *testing.T) {
	// Given
	text := "12345"

	// When
	got := EstimateTokens(text)

	// Then
	if got != 2 {
		t.Fatalf("tokens = %d, want 2", got)
	}
}
