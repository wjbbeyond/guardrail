package cost

import (
	"context"
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
