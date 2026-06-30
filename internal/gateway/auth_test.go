package gateway

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wjbbeyond/guardrail/internal/authn"
	"github.com/wjbbeyond/guardrail/internal/config"
)

func TestServer_ChatCompletions_rejectsRequest_whenProxyKeyIsMissing(t *testing.T) {
	// Given
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("upstream should not be called")
	}))
	defer upstream.Close()
	server, _ := newTestServerWithConfig(t, ctx, upstream.URL, 10, func(cfg *config.Config) {
		cfg.Auth = config.AuthConfig{
			Enabled:      true,
			ProxyAPIKeys: []string{"proxy-key"},
			AdminAPIKeys: []string{"admin-key"},
		}
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`))
	rec := httptest.NewRecorder()

	// When
	server.Handler().ServeHTTP(rec, req)

	// Then
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", rec.Code, rec.Body.String())
	}
}

func TestServer_ChatCompletions_acceptsRequest_whenProxyKeyMatches(t *testing.T) {
	// Given
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl-test","object":"chat.completion","model":"gpt-4o-mini","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"completion_tokens":3}}`)
	}))
	defer upstream.Close()
	server, _ := newTestServerWithConfig(t, ctx, upstream.URL, 10, func(cfg *config.Config) {
		cfg.Auth = config.AuthConfig{
			Enabled:      true,
			ProxyAPIKeys: []string{"proxy-key"},
			AdminAPIKeys: []string{"admin-key"},
		}
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer proxy-key")
	rec := httptest.NewRecorder()

	// When
	server.Handler().ServeHTTP(rec, req)

	// Then
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
}

func TestServer_AdminEndpoints_requireAdminKey_whenAuthEnabled(t *testing.T) {
	// Given
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("upstream should not be called")
	}))
	defer upstream.Close()
	server, _ := newTestServerWithConfig(t, ctx, upstream.URL, 10, func(cfg *config.Config) {
		cfg.Auth = config.AuthConfig{
			Enabled:      true,
			ProxyAPIKeys: []string{"proxy-key"},
			AdminAPIKeys: []string{"admin-key"},
		}
	})

	// When
	missingKey := httptest.NewRecorder()
	server.Handler().ServeHTTP(missingKey, httptest.NewRequest(http.MethodGet, "/v1/admin/costs", nil))
	proxyKey := httptest.NewRecorder()
	proxyReq := httptest.NewRequest(http.MethodGet, "/v1/admin/costs", nil)
	proxyReq.Header.Set("Authorization", "Bearer proxy-key")
	server.Handler().ServeHTTP(proxyKey, proxyReq)
	adminKey := httptest.NewRecorder()
	adminReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	adminReq.Header.Set(authn.AdminAPIKeyHeader, "admin-key")
	server.Handler().ServeHTTP(adminKey, adminReq)

	// Then
	if missingKey.Code != http.StatusUnauthorized {
		t.Fatalf("missing key status = %d, want 401", missingKey.Code)
	}
	if proxyKey.Code != http.StatusUnauthorized {
		t.Fatalf("proxy key status = %d, want 401", proxyKey.Code)
	}
	if adminKey.Code != http.StatusOK {
		t.Fatalf("admin key status = %d, want 200; body=%s", adminKey.Code, adminKey.Body.String())
	}
}

func TestServer_ChatCompletions_blocksTenant_whenTenantBudgetExceeded(t *testing.T) {
	// Given
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("upstream should not be called")
	}))
	defer upstream.Close()
	server, _ := newTestServerWithConfig(t, ctx, upstream.URL, 10, func(cfg *config.Config) {
		cfg.Auth = config.AuthConfig{
			Enabled:      true,
			AdminAPIKeys: []string{"admin-key"},
		}
		cfg.Tenants = []config.TenantConfig{{
			ID:                  "limited",
			ProxyAPIKeys:        []string{"limited-key"},
			DailyBudgetUSD:      1,
			PerRequestBudgetUSD: 0.000001,
		}}
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"role":"user","content":"hello"}],"max_tokens":1000}`))
	req.Header.Set(authn.ProxyAPIKeyHeader, "limited-key")
	rec := httptest.NewRecorder()

	// When
	server.Handler().ServeHTTP(rec, req)

	// Then
	if rec.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want 402; body=%s", rec.Code, rec.Body.String())
	}
}

func TestServer_ChatCompletions_rateLimitsTenant_whenBurstIsExhausted(t *testing.T) {
	// Given
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl-test","object":"chat.completion","model":"gpt-4o-mini","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"completion_tokens":3}}`)
	}))
	defer upstream.Close()
	server, _ := newTestServerWithConfig(t, ctx, upstream.URL, 10, func(cfg *config.Config) {
		cfg.Auth = config.AuthConfig{
			Enabled:      true,
			AdminAPIKeys: []string{"admin-key"},
		}
		cfg.RateLimit = config.RateLimitConfig{Enabled: true, RequestsPerMinute: 1, Burst: 1}
		cfg.Tenants = []config.TenantConfig{{
			ID:           "limited",
			ProxyAPIKeys: []string{"limited-key"},
		}}
	})
	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	firstReq.Header.Set(authn.ProxyAPIKeyHeader, "limited-key")
	secondReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	secondReq.Header.Set(authn.ProxyAPIKeyHeader, "limited-key")
	first := httptest.NewRecorder()
	second := httptest.NewRecorder()

	// When
	server.Handler().ServeHTTP(first, firstReq)
	server.Handler().ServeHTTP(second, secondReq)

	// Then
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body=%s", first.Code, first.Body.String())
	}
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429; body=%s", second.Code, second.Body.String())
	}
}

func TestServer_ChatCompletions_acceptsOIDCTenantToken_whenVerifierSucceeds(t *testing.T) {
	// Given
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"id":"chatcmpl-test","object":"chat.completion","model":"gpt-4o-mini","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"completion_tokens":3}}`)
	}))
	defer upstream.Close()
	server, store := newTestServerWithConfigAndVerifier(t, ctx, upstream.URL, 10, func(cfg *config.Config) {
		cfg.Auth = config.AuthConfig{
			Enabled:      true,
			AdminAPIKeys: []string{"admin-key"},
			OIDC: config.OIDCConfig{
				Enabled:     true,
				IssuerURL:   "https://issuer.example.com",
				ClientID:    "guardrail",
				TenantClaim: "tenant",
			},
		}
		cfg.Tenants = []config.TenantConfig{{ID: "acme"}}
	}, fakeTokenVerifier{token: authn.Token{
		Subject: "user-1",
		Claims:  map[string]any{"tenant": "acme"},
	}})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("Authorization", "Bearer oidc-token")
	rec := httptest.NewRecorder()

	// When
	server.Handler().ServeHTTP(rec, req)

	// Then
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	events, err := store.Recent(ctx, 10)
	if err != nil {
		t.Fatalf("read audit events: %v", err)
	}
	if events[0].TenantID != "acme" {
		t.Fatalf("audit tenant = %s, want acme", events[0].TenantID)
	}
}
