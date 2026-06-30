package cost

import (
	"context"
	"fmt"
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

type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
}

type Decision struct {
	Allowed  bool    `json:"allowed"`
	Reason   string  `json:"reason,omitempty"`
	TenantID string  `json:"tenant_id"`
	Spent    float64 `json:"spent_usd"`
	Limit    float64 `json:"limit_usd"`
}

type Snapshot struct {
	TenantID         string  `json:"tenant_id"`
	Day              string  `json:"day"`
	SpentUSD         float64 `json:"spent_usd"`
	DailyBudgetUSD   float64 `json:"daily_budget_usd"`
	RequestBudgetUSD float64 `json:"request_budget_usd"`
}

type Budget struct {
	Daily  float64
	PerReq float64
}

type Pricer interface {
	Price(model string, promptTokens int, completionTokens int) float64
}

type spendLedger interface {
	AddSpend(ctx context.Context, tenantID string, day string, amount float64) error
	Spend(ctx context.Context, tenantID string, day string) (float64, error)
}

type Tracker struct {
	mu         sync.Mutex
	clock      Clock
	defaults   Budget
	budgets    map[string]Budget
	spendByDay map[string]float64
	ledger     spendLedger
	pricer     Pricer
}

type TrackerOptions struct {
	Cost    config.CostConfig
	Tenants []config.TenantConfig
	Clock   Clock
	Ledger  spendLedger
	Pricer  Pricer
}

func NewTracker(cfg config.CostConfig, clock Clock) *Tracker {
	return NewTrackerWithLedger(cfg, clock, nil)
}

func NewTrackerWithLedger(cfg config.CostConfig, clock Clock, ledger spendLedger) *Tracker {
	return NewTrackerWithOptions(TrackerOptions{Cost: cfg, Clock: clock, Ledger: ledger})
}

func NewTrackerWithOptions(options TrackerOptions) *Tracker {
	clock := options.Clock
	if clock == nil {
		clock = RealClock{}
	}
	pricer := options.Pricer
	if pricer == nil {
		pricer = StaticPricer{}
	}
	return &Tracker{
		clock:      clock,
		defaults:   Budget{Daily: options.Cost.DailyBudgetUSD, PerReq: options.Cost.PerRequestBudgetUSD},
		budgets:    tenantBudgets(options.Cost, options.Tenants),
		spendByDay: make(map[string]float64),
		ledger:     options.Ledger,
		pricer:     pricer,
	}
}

func (t *Tracker) Allow(ctx context.Context, model string, promptTokens int, maxCompletionTokens int) (Decision, error) {
	return t.AllowTenant(ctx, authn.DefaultTenantID, model, promptTokens, maxCompletionTokens)
}

func (t *Tracker) AllowTenant(ctx context.Context, tenantID string, model string, promptTokens int, maxCompletionTokens int) (Decision, error) {
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	estimated := t.pricer.Price(model, promptTokens, maxCompletionTokens)
	t.mu.Lock()
	defer t.mu.Unlock()

	budget := t.budgetForTenant(tenantID)
	day := t.day()
	spent, err := t.spentForDay(ctx, tenantID, day)
	if err != nil {
		return Decision{}, err
	}
	if budget.PerReq > 0 && estimated > budget.PerReq {
		return Decision{Allowed: false, Reason: "per_request_budget_exceeded", TenantID: tenantID, Spent: estimated, Limit: budget.PerReq}, nil
	}
	if budget.Daily > 0 && spent+estimated > budget.Daily {
		return Decision{Allowed: false, Reason: "daily_budget_exceeded", TenantID: tenantID, Spent: spent, Limit: budget.Daily}, nil
	}
	return Decision{Allowed: true, TenantID: tenantID, Spent: spent, Limit: budget.Daily}, nil
}

func (t *Tracker) Record(ctx context.Context, model string, promptTokens int, completionTokens int) (Usage, error) {
	return t.RecordTenant(ctx, authn.DefaultTenantID, model, promptTokens, completionTokens)
}

func (t *Tracker) RecordTenant(ctx context.Context, tenantID string, model string, promptTokens int, completionTokens int) (Usage, error) {
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	usage := Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		CostUSD:          t.pricer.Price(model, promptTokens, completionTokens),
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	day := t.day()
	if t.ledger != nil {
		if err := t.ledger.AddSpend(ctx, tenantID, day, usage.CostUSD); err != nil {
			return Usage{}, fmt.Errorf("record spend: %w", err)
		}
		spent, err := t.ledger.Spend(ctx, tenantID, day)
		if err != nil {
			return Usage{}, fmt.Errorf("read recorded spend: %w", err)
		}
		t.spendByDay[spendKey(tenantID, day)] = spent
		return usage, nil
	}
	t.spendByDay[spendKey(tenantID, day)] += usage.CostUSD
	return usage, nil
}

func (t *Tracker) Snapshot(ctx context.Context) (Snapshot, error) {
	return t.SnapshotTenant(ctx, authn.DefaultTenantID)
}

func (t *Tracker) SnapshotTenant(ctx context.Context, tenantID string) (Snapshot, error) {
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	day := t.day()
	spent, err := t.spentForDay(ctx, tenantID, day)
	if err != nil {
		return Snapshot{}, err
	}
	budget := t.budgetForTenant(tenantID)
	return Snapshot{
		TenantID:         tenantID,
		Day:              day,
		SpentUSD:         spent,
		DailyBudgetUSD:   budget.Daily,
		RequestBudgetUSD: budget.PerReq,
	}, nil
}

func (t *Tracker) day() string {
	return t.clock.Now().Format("2006-01-02")
}

func (t *Tracker) spentForDay(ctx context.Context, tenantID string, day string) (float64, error) {
	key := spendKey(tenantID, day)
	if t.ledger == nil {
		return t.spendByDay[key], nil
	}
	spent, err := t.ledger.Spend(ctx, tenantID, day)
	if err != nil {
		return 0, fmt.Errorf("read spend: %w", err)
	}
	t.spendByDay[key] = spent
	return spent, nil
}

func (t *Tracker) budgetForTenant(tenantID string) Budget {
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	if budget, ok := t.budgets[tenantID]; ok {
		return budget
	}
	return t.defaults
}

func tenantBudgets(cfg config.CostConfig, tenants []config.TenantConfig) map[string]Budget {
	budgets := make(map[string]Budget, len(tenants))
	defaults := Budget{Daily: cfg.DailyBudgetUSD, PerReq: cfg.PerRequestBudgetUSD}
	for _, tenant := range tenants {
		budget := defaults
		if tenant.DailyBudgetUSD > 0 {
			budget.Daily = tenant.DailyBudgetUSD
		}
		if tenant.PerRequestBudgetUSD > 0 {
			budget.PerReq = tenant.PerRequestBudgetUSD
		}
		budgets[tenant.ID] = budget
	}
	return budgets
}

func spendKey(tenantID string, day string) string {
	if tenantID == "" {
		tenantID = authn.DefaultTenantID
	}
	return tenantID + "\x00" + day
}
