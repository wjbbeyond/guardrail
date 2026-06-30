package ratelimit

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

func TestLimiter_Allow_rejectsRequest_whenTenantBurstIsExhausted(t *testing.T) {
	// Given
	clock := fakeClock{now: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	limiter := New(config.RateLimitConfig{Enabled: true, RequestsPerMinute: 60, Burst: 1}, nil, clock)

	// When
	first := limiter.Allow("acme")
	second := limiter.Allow("acme")

	// Then
	if !first.Allowed {
		t.Fatal("first request rejected, want allowed")
	}
	if second.Allowed {
		t.Fatal("second request allowed, want rejected")
	}
	if second.Reason != "rate_limit_exceeded" {
		t.Fatalf("reason = %s, want rate_limit_exceeded", second.Reason)
	}
}

func TestLimiter_Allow_usesTenantOverride(t *testing.T) {
	// Given
	clock := fakeClock{now: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)}
	limiter := New(
		config.RateLimitConfig{Enabled: true, RequestsPerMinute: 60, Burst: 10},
		[]config.TenantConfig{{
			ID:        "limited",
			RateLimit: config.RateLimitConfig{Enabled: true, RequestsPerMinute: 1, Burst: 1},
		}},
		clock,
	)

	// When
	first := limiter.Allow("limited")
	second := limiter.Allow("limited")

	// Then
	if !first.Allowed {
		t.Fatal("first request rejected, want allowed")
	}
	if second.Allowed {
		t.Fatal("second request allowed, want tenant override rejection")
	}
}
