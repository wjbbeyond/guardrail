package ratelimit

import (
	"sync"
	"time"

	"github.com/wjbbeyond/guardrail/internal/authn"
	"github.com/wjbbeyond/guardrail/internal/config"
)

type Clock interface {
	Now() time.Time
}

type RealClock struct{}

func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

type Decision struct {
	Allowed    bool   `json:"allowed"`
	Reason     string `json:"reason,omitempty"`
	TenantID   string `json:"tenant_id"`
	RetryAfter int    `json:"retry_after_seconds,omitempty"`
}

type Limit struct {
	Enabled           bool
	RequestsPerMinute int
	Burst             int
}

type Limiter struct {
	mu           sync.Mutex
	clock        Clock
	defaultLimit Limit
	tenantLimits map[string]Limit
	buckets      map[string]bucket
}

type bucket struct {
	tokens  float64
	updated time.Time
}

func New(defaultCfg config.RateLimitConfig, tenants []config.TenantConfig, clock Clock) *Limiter {
	limits := make(map[string]Limit, len(tenants))
	defaultLimit := limitFromConfig(defaultCfg)
	for _, tenant := range tenants {
		limits[tenant.ID] = mergeLimit(defaultLimit, limitFromConfig(tenant.RateLimit))
	}
	return &Limiter{
		clock:        clock,
		defaultLimit: defaultLimit,
		tenantLimits: limits,
		buckets:      make(map[string]bucket),
	}
}

func (l *Limiter) Allow(tenantID string) Decision {
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	limit := l.limitForTenant(tenantID)
	if !limit.Enabled {
		return Decision{Allowed: true, TenantID: tenantID}
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	current := l.buckets[tenantID]
	if current.updated.IsZero() {
		current = bucket{tokens: float64(limit.Burst), updated: now}
	}
	elapsed := now.Sub(current.updated).Minutes()
	current.tokens += elapsed * float64(limit.RequestsPerMinute)
	if current.tokens > float64(limit.Burst) {
		current.tokens = float64(limit.Burst)
	}
	current.updated = now
	if current.tokens >= 1 {
		current.tokens--
		l.buckets[tenantID] = current
		return Decision{Allowed: true, TenantID: tenantID}
	}
	l.buckets[tenantID] = current
	return Decision{
		Allowed:    false,
		Reason:     "rate_limit_exceeded",
		TenantID:   tenantID,
		RetryAfter: retryAfterSeconds(limit.RequestsPerMinute, current.tokens),
	}
}

func (l *Limiter) limitForTenant(tenantID string) Limit {
	if limit, ok := l.tenantLimits[tenantID]; ok {
		return limit
	}
	return l.defaultLimit
}

func limitFromConfig(cfg config.RateLimitConfig) Limit {
	enabled := cfg.Enabled || cfg.RequestsPerMinute > 0 || cfg.Burst > 0
	return Limit{
		Enabled:           enabled,
		RequestsPerMinute: cfg.RequestsPerMinute,
		Burst:             cfg.Burst,
	}
}

func mergeLimit(base Limit, override Limit) Limit {
	if !override.Enabled {
		return base
	}
	if override.RequestsPerMinute == 0 {
		override.RequestsPerMinute = base.RequestsPerMinute
	}
	if override.Burst == 0 {
		override.Burst = base.Burst
	}
	return override
}

func retryAfterSeconds(requestsPerMinute int, tokens float64) int {
	if requestsPerMinute <= 0 {
		return 60
	}
	seconds := int(((1 - tokens) / float64(requestsPerMinute)) * 60)
	if seconds < 1 {
		return 1
	}
	return seconds
}
