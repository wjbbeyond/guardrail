package config

import (
	"strings"
	"testing"
)

func TestConfig_Validate_rejectsEnabledAuthWithoutKeys(t *testing.T) {
	// Given
	cfg := Default()

	// When
	err := cfg.Validate()

	// Then
	if err == nil {
		t.Fatal("expected auth validation error")
	}
	if !strings.Contains(err.Error(), "auth.admin_api_keys") {
		t.Fatalf("error = %v, want admin key validation", err)
	}
}

func TestConfig_Validate_acceptsEnabledAuthWithProxyAndAdminKeys(t *testing.T) {
	// Given
	cfg := Default()
	cfg.Auth.ProxyAPIKeys = []string{"proxy-key"}
	cfg.Auth.AdminAPIKeys = []string{"admin-key"}

	// When
	err := cfg.Validate()

	// Then
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestConfig_Validate_acceptsEnabledAuthWithTenantProxyKeys(t *testing.T) {
	// Given
	cfg := Default()
	cfg.Auth.AdminAPIKeys = []string{"admin-key"}
	cfg.Tenants = []TenantConfig{{
		ID:           "acme",
		ProxyAPIKeys: []string{"proxy-key"},
	}}

	// When
	err := cfg.Validate()

	// Then
	if err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}

func TestConfig_Validate_rejectsEnabledOIDCWithoutIssuer(t *testing.T) {
	// Given
	cfg := Default()
	cfg.Auth.AdminAPIKeys = []string{"admin-key"}
	cfg.Auth.OIDC.Enabled = true
	cfg.Auth.OIDC.ClientID = "guardrail"

	// When
	err := cfg.Validate()

	// Then
	if err == nil {
		t.Fatal("expected oidc issuer validation error")
	}
	if !strings.Contains(err.Error(), "auth.oidc.issuer_url") {
		t.Fatalf("error = %v, want oidc issuer validation", err)
	}
}

func TestConfig_Validate_rejectsInvalidRateLimit(t *testing.T) {
	// Given
	cfg := Default()
	cfg.Auth.ProxyAPIKeys = []string{"proxy-key"}
	cfg.Auth.AdminAPIKeys = []string{"admin-key"}
	cfg.RateLimit = RateLimitConfig{Enabled: true, Burst: 1}

	// When
	err := cfg.Validate()

	// Then
	if err == nil {
		t.Fatal("expected rate limit validation error")
	}
	if !strings.Contains(err.Error(), "requests_per_minute") {
		t.Fatalf("error = %v, want requests_per_minute validation", err)
	}
}

func TestConfig_ApplyEnv_addsAuthKeysFromEnvironment(t *testing.T) {
	// Given
	t.Setenv("GUARDRAIL_PROXY_API_KEYS", "proxy-a, proxy-b")
	t.Setenv("GUARDRAIL_ADMIN_API_KEY", "admin-a")
	cfg := Default()

	// When
	applyEnv(&cfg)

	// Then
	if len(cfg.Auth.ProxyAPIKeys) != 2 {
		t.Fatalf("proxy keys = %d, want 2", len(cfg.Auth.ProxyAPIKeys))
	}
	if len(cfg.Auth.AdminAPIKeys) != 1 {
		t.Fatalf("admin keys = %d, want 1", len(cfg.Auth.AdminAPIKeys))
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v, want nil", err)
	}
}
