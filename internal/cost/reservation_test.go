package cost

import (
	"context"
	"github.com/wjbbeyond/guardrail/internal/config"
	"math"
	"sync"
	"testing"
	"time"
)

func TestReservationsConcurrentPersistentAndIdempotent(t *testing.T) {
	ctx := context.Background()
	dsn := "file:" + t.TempDir() + "/cost.db?_pragma=busy_timeout(5000)"
	first, err := OpenSQLiteLedger(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := OpenSQLiteLedger(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	cfg := config.CostConfig{DailyBudgetUSD: Price("gpt-4o", 100, 100) * 1.5}
	clock := fakeClock{now: time.Date(2026, 9, 30, 23, 59, 0, 0, time.UTC)}
	trackers := []*Tracker{NewTrackerWithLedger(cfg, clock, first), NewTrackerWithLedger(cfg, clock, second)}
	var wg sync.WaitGroup
	accepted := make(chan Reservation, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, d, e := trackers[i%2].ReserveTenant(ctx, "tenant", "gpt-4o", 100, 100)
			if e != nil {
				t.Error(e)
			}
			if d.Allowed {
				accepted <- r
			}
		}(i)
	}
	wg.Wait()
	close(accepted)
	var reservations []Reservation
	for r := range accepted {
		reservations = append(reservations, r)
	}
	if len(reservations) != 1 {
		t.Fatalf("accepted %d requests", len(reservations))
	}
	restarted := NewTrackerWithLedger(cfg, clock, second)
	_, d, e := restarted.ReserveTenant(ctx, "tenant", "gpt-4o", 100, 100)
	if e != nil || d.Allowed {
		t.Fatalf("restart lost reservation: %+v %v", d, e)
	}
	r := reservations[0]
	// Settlement remains on the reservation's UTC day, even after midnight.
	restarted.clock = fakeClock{now: clock.now.Add(2 * time.Minute)}
	for i := 0; i < 2; i++ {
		if _, err := restarted.Settle(ctx, r, "gpt-4o", 10, 10); err != nil {
			t.Fatal(err)
		}
	}
	spent, err := first.Spend(ctx, "tenant", r.Day)
	if err != nil || math.Abs(spent-Price("gpt-4o", 10, 10)) > 1e-12 {
		t.Fatalf("spent %v error %v", spent, err)
	}
}

func TestUnknownModelRequiresPrice(t *testing.T) {
	tracker := NewTracker(config.CostConfig{DailyBudgetUSD: 10}, RealClock{})
	_, decision, err := tracker.ReserveTenant(context.Background(), "tenant", "unpriced-model", 10, 10)
	if err != nil || decision.Allowed || decision.Reason != "model_price_required" {
		t.Fatalf("%+v %v", decision, err)
	}
}
