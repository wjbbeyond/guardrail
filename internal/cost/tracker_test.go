package cost

import (
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
	tracker := NewTracker(config.CostConfig{DailyBudgetUSD: 0.000001, PerRequestBudgetUSD: 1}, fakeClock{now: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)})
	tracker.Record("gpt-4o", 1000, 1000)

	// When
	decision := tracker.Allow("gpt-4o", 1000, 1000)

	// Then
	if decision.Allowed {
		t.Fatal("expected request to be rejected")
	}
	if decision.Reason != "daily_budget_exceeded" {
		t.Fatalf("reason = %s, want daily_budget_exceeded", decision.Reason)
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
