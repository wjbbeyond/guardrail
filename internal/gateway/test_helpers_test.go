package gateway

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/wjbbeyond/guardrail/internal/audit"
	"github.com/wjbbeyond/guardrail/internal/authn"
	"github.com/wjbbeyond/guardrail/internal/config"
	"github.com/wjbbeyond/guardrail/internal/cost"
	"github.com/wjbbeyond/guardrail/internal/metrics"
	"github.com/wjbbeyond/guardrail/internal/provider"
	"github.com/wjbbeyond/guardrail/internal/ratelimit"
	"github.com/wjbbeyond/guardrail/internal/security"
)

type fakeTokenVerifier struct {
	token authn.Token
}

func (v fakeTokenVerifier) Verify(_ context.Context, _ string) (authn.Token, error) {
	return v.token, nil
}

func newTestServer(t *testing.T, ctx context.Context, upstreamURL string, dailyBudget float64) (*Server, *audit.Store) {
	t.Helper()
	return newTestServerWithConfig(t, ctx, upstreamURL, dailyBudget, nil)
}

func newTestServerWithConfig(t *testing.T, ctx context.Context, upstreamURL string, dailyBudget float64, mutate func(*config.Config)) (*Server, *audit.Store) {
	t.Helper()
	return newTestServerWithConfigAndVerifier(t, ctx, upstreamURL, dailyBudget, mutate, nil)
}

func newTestServerWithConfigAndVerifier(t *testing.T, ctx context.Context, upstreamURL string, dailyBudget float64, mutate func(*config.Config), verifier authn.TokenVerifier) (*Server, *audit.Store) {
	t.Helper()
	cfg := config.Default()
	cfg.Server.ListenAddr = "127.0.0.1:0"
	cfg.Auth.Enabled = false
	cfg.Audit.SQLiteDSN = "file:" + t.TempDir() + "/audit.db?_pragma=busy_timeout(5000)"
	cfg.Cost.DailyBudgetUSD = dailyBudget
	cfg.Cost.PerRequestBudgetUSD = 10
	cfg.Security.PromptInjectionMode = "block"
	cfg.Security.PIIMode = "redact"
	cfg.Providers = []config.ProviderConfig{{
		Name:    "mock",
		Type:    config.ProviderOpenAICompatible,
		BaseURL: upstreamURL,
		APIKeys: []string{"test-key"},
		Models:  []string{"gpt-4o", "gpt-4o-mini"},
	}}
	if mutate != nil {
		mutate(&cfg)
	}
	store, err := audit.Open(ctx, cfg.Audit.SQLiteDSN)
	if err != nil {
		t.Fatalf("open audit: %v", err)
	}
	t.Cleanup(store.Close)
	router, err := provider.NewRouter(cfg.Providers, time.Second)
	if err != nil {
		t.Fatalf("new router: %v", err)
	}
	server := New(Dependencies{
		Config: cfg,
		Auth:   authn.NewManagerWithVerifier(cfg, verifier),
		Router: router,
		Guard:  security.NewGuard(cfg.Security),
		Costs: cost.NewTrackerWithOptions(cost.TrackerOptions{
			Cost:    cfg.Cost,
			Tenants: cfg.Tenants,
			Clock:   cost.RealClock{},
		}),
		Limits:  ratelimit.New(cfg.RateLimit, cfg.Tenants, ratelimit.RealClock{}),
		Audit:   store,
		Metrics: metrics.NewRegistry(),
		Logger:  slog.New(slog.NewTextHandler(os.Stderr, nil)),
	})
	return server, store
}
