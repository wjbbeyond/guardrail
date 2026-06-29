package cost

import (
	"encoding/json"
	"sync"
	"time"

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
	Allowed bool    `json:"allowed"`
	Reason  string  `json:"reason,omitempty"`
	Spent   float64 `json:"spent_usd"`
	Limit   float64 `json:"limit_usd"`
}

type Snapshot struct {
	Day              string  `json:"day"`
	SpentUSD         float64 `json:"spent_usd"`
	DailyBudgetUSD   float64 `json:"daily_budget_usd"`
	RequestBudgetUSD float64 `json:"request_budget_usd"`
}

type Tracker struct {
	mu         sync.Mutex
	clock      Clock
	daily      float64
	perReq     float64
	spendByDay map[string]float64
}

func NewTracker(cfg config.CostConfig, clock Clock) *Tracker {
	return &Tracker{
		clock:      clock,
		daily:      cfg.DailyBudgetUSD,
		perReq:     cfg.PerRequestBudgetUSD,
		spendByDay: make(map[string]float64),
	}
}

func (t *Tracker) Allow(model string, promptTokens int, maxCompletionTokens int) Decision {
	estimated := Price(model, promptTokens, maxCompletionTokens)
	t.mu.Lock()
	defer t.mu.Unlock()

	day := t.day()
	spent := t.spendByDay[day]
	if t.perReq > 0 && estimated > t.perReq {
		return Decision{Allowed: false, Reason: "per_request_budget_exceeded", Spent: estimated, Limit: t.perReq}
	}
	if t.daily > 0 && spent+estimated > t.daily {
		return Decision{Allowed: false, Reason: "daily_budget_exceeded", Spent: spent, Limit: t.daily}
	}
	return Decision{Allowed: true, Spent: spent, Limit: t.daily}
}

func (t *Tracker) Record(model string, promptTokens int, completionTokens int) Usage {
	usage := Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		CostUSD:          Price(model, promptTokens, completionTokens),
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.spendByDay[t.day()] += usage.CostUSD
	return usage
}

func (t *Tracker) Snapshot() Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	day := t.day()
	return Snapshot{
		Day:              day,
		SpentUSD:         t.spendByDay[day],
		DailyBudgetUSD:   t.daily,
		RequestBudgetUSD: t.perReq,
	}
}

func (t *Tracker) day() string {
	return t.clock.Now().Format("2006-01-02")
}

func EstimateTokens(text string) int {
	runes := len([]rune(text))
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}

func CompletionTokensFromOpenAI(body []byte) int {
	var payload struct {
		Usage struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return EstimateTokens(string(body))
	}
	if payload.Usage.CompletionTokens > 0 {
		return payload.Usage.CompletionTokens
	}
	return EstimateTokens(string(body))
}

func Price(model string, promptTokens int, completionTokens int) float64 {
	price := priceForModel(model)
	return (float64(promptTokens)*price.inputPerMTok + float64(completionTokens)*price.outputPerMTok) / 1_000_000
}

type modelPrice struct {
	inputPerMTok  float64
	outputPerMTok float64
}

func priceForModel(model string) modelPrice {
	prices := map[string]modelPrice{
		"gpt-4o":                {inputPerMTok: 2.50, outputPerMTok: 10.00},
		"gpt-4o-mini":           {inputPerMTok: 0.15, outputPerMTok: 0.60},
		"gpt-4.1":               {inputPerMTok: 2.00, outputPerMTok: 8.00},
		"gpt-4.1-mini":          {inputPerMTok: 0.40, outputPerMTok: 1.60},
		"claude-3-5-sonnet":     {inputPerMTok: 3.00, outputPerMTok: 15.00},
		"claude-3-5-haiku":      {inputPerMTok: 0.80, outputPerMTok: 4.00},
		"gemini-1.5-pro":        {inputPerMTok: 1.25, outputPerMTok: 5.00},
		"gemini-1.5-flash":      {inputPerMTok: 0.075, outputPerMTok: 0.30},
		"gemini-2.0-flash":      {inputPerMTok: 0.10, outputPerMTok: 0.40},
		"gemini-2.5-flash-lite": {inputPerMTok: 0.10, outputPerMTok: 0.40},
		"gemini-2.5-pro":        {inputPerMTok: 1.25, outputPerMTok: 10.00},
	}
	if price, ok := prices[model]; ok {
		return price
	}
	return modelPrice{inputPerMTok: 1.00, outputPerMTok: 3.00}
}
