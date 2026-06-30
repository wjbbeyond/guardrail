package cost

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wjbbeyond/guardrail/internal/config"
)

func TestPriceTable_Refresh_updatesModelPricingFromJSONFeed(t *testing.T) {
	// Given
	ctx := context.Background()
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"models":{"demo-model":{"input_per_mtok":2,"output_per_mtok":4}}}`))
	}))
	defer feed.Close()
	table := NewPriceTable(config.PricingConfig{URL: feed.URL})

	// When
	err := table.Refresh(ctx)

	// Then
	if err != nil {
		t.Fatalf("Refresh() error = %v, want nil", err)
	}
	got := table.Price("demo-model", 1_000_000, 1_000_000)
	if got != 6 {
		t.Fatalf("price = %.2f, want 6.00", got)
	}
}
