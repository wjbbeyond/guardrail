package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/wjbbeyond/guardrail/internal/config"
)

type PriceTable struct {
	mu     sync.RWMutex
	url    string
	client *http.Client
	prices map[string]modelPrice
}

type PriceFeed struct {
	Models map[string]ModelPrice `json:"models"`
}

type ModelPrice struct {
	InputPerMTok  float64 `json:"input_per_mtok"`
	OutputPerMTok float64 `json:"output_per_mtok"`
}

func NewPriceTable(cfg config.PricingConfig) *PriceTable {
	return &PriceTable{
		url:    cfg.URL,
		client: &http.Client{Timeout: 10 * time.Second},
		prices: defaultPrices(),
	}
}

func (p *PriceTable) Price(model string, promptTokens int, completionTokens int) float64 {
	p.mu.RLock()
	price, ok := p.prices[model]
	p.mu.RUnlock()
	if !ok {
		price = modelPrice{inputPerMTok: 1.00, outputPerMTok: 3.00}
	}
	return (float64(promptTokens)*price.inputPerMTok + float64(completionTokens)*price.outputPerMTok) / 1_000_000
}

func (p *PriceTable) Refresh(ctx context.Context) error {
	if p.url == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return fmt.Errorf("build pricing request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch pricing table: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("fetch pricing table: status %d", resp.StatusCode)
	}
	var feed PriceFeed
	if err := json.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return fmt.Errorf("decode pricing table: %w", err)
	}
	if len(feed.Models) == 0 {
		return fmt.Errorf("decode pricing table: no models")
	}
	next := defaultPrices()
	for model, price := range feed.Models {
		next[model] = modelPrice{inputPerMTok: price.InputPerMTok, outputPerMTok: price.OutputPerMTok}
	}
	p.mu.Lock()
	p.prices = next
	p.mu.Unlock()
	return nil
}

func (p *PriceTable) RunAutoRefresh(ctx context.Context, interval time.Duration, onError func(error)) {
	if p.url == "" || interval <= 0 {
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.Refresh(ctx); err != nil && onError != nil {
				onError(err)
			}
		}
	}
}

func defaultPrices() map[string]modelPrice {
	return map[string]modelPrice{
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
}
