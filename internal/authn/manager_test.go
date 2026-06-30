package authn

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/wjbbeyond/guardrail/internal/config"
)

type fakeVerifier struct {
	token Token
}

func (v fakeVerifier) Verify(_ context.Context, _ string) (Token, error) {
	return v.token, nil
}

func TestManager_AuthenticateProxy_mapsTenantAPIKeyToTenant(t *testing.T) {
	// Given
	cfg := config.Default()
	cfg.Auth.ProxyAPIKeys = nil
	cfg.Auth.AdminAPIKeys = []string{"admin-key"}
	cfg.Tenants = []config.TenantConfig{{
		ID:           "acme",
		ProxyAPIKeys: []string{"proxy-key"},
	}}
	manager := NewManagerWithVerifier(cfg, nil)
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set(ProxyAPIKeyHeader, "proxy-key")

	// When
	identity, ok, err := manager.AuthenticateProxy(context.Background(), req)

	// Then
	if err != nil {
		t.Fatalf("AuthenticateProxy() error = %v, want nil", err)
	}
	if !ok {
		t.Fatal("AuthenticateProxy() ok = false, want true")
	}
	if identity.TenantID != "acme" {
		t.Fatalf("tenant = %s, want acme", identity.TenantID)
	}
}

func TestManager_AuthenticateProxy_mapsOIDCTenantClaimToTenant(t *testing.T) {
	// Given
	cfg := config.Default()
	cfg.Auth.ProxyAPIKeys = nil
	cfg.Auth.AdminAPIKeys = []string{"admin-key"}
	cfg.Auth.OIDC.Enabled = true
	cfg.Auth.OIDC.TenantClaim = "org_id"
	cfg.Tenants = []config.TenantConfig{{ID: "acme"}}
	manager := NewManagerWithVerifier(cfg, fakeVerifier{token: Token{
		Subject: "user-1",
		Claims:  map[string]any{"org_id": "acme"},
	}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer oidc-token")

	// When
	identity, ok, err := manager.AuthenticateProxy(context.Background(), req)

	// Then
	if err != nil {
		t.Fatalf("AuthenticateProxy() error = %v, want nil", err)
	}
	if !ok {
		t.Fatal("AuthenticateProxy() ok = false, want true")
	}
	if identity.TenantID != "acme" {
		t.Fatalf("tenant = %s, want acme", identity.TenantID)
	}
	if identity.Subject != "user-1" {
		t.Fatalf("subject = %s, want user-1", identity.Subject)
	}
}

func TestManager_AuthenticateProxy_rejectsOIDCTokenWithoutTenantClaim_whenTenantsAreConfigured(t *testing.T) {
	// Given
	cfg := config.Default()
	cfg.Auth.ProxyAPIKeys = nil
	cfg.Auth.AdminAPIKeys = []string{"admin-key"}
	cfg.Auth.OIDC.Enabled = true
	cfg.Auth.OIDC.TenantClaim = "org_id"
	cfg.Tenants = []config.TenantConfig{{ID: "acme"}}
	manager := NewManagerWithVerifier(cfg, fakeVerifier{token: Token{
		Subject: "user-1",
		Claims:  map[string]any{},
	}})
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer oidc-token")

	// When
	_, ok, err := manager.AuthenticateProxy(context.Background(), req)

	// Then
	if err == nil {
		t.Fatal("AuthenticateProxy() error = nil, want missing tenant claim error")
	}
	if ok {
		t.Fatal("AuthenticateProxy() ok = true, want false")
	}
}

func TestManager_AuthenticateAdmin_acceptsOIDCAdminGroup(t *testing.T) {
	// Given
	cfg := config.Default()
	cfg.Auth.ProxyAPIKeys = []string{"proxy-key"}
	cfg.Auth.AdminAPIKeys = []string{"admin-key"}
	cfg.Auth.OIDC.Enabled = true
	cfg.Auth.OIDC.AdminGroups = []string{"guardrail-admins"}
	manager := NewManagerWithVerifier(cfg, fakeVerifier{token: Token{
		Subject: "admin-1",
		Claims:  map[string]any{"groups": []any{"guardrail-admins"}},
	}})
	req := httptest.NewRequest("GET", "/v1/admin/costs", nil)
	req.Header.Set("Authorization", "Bearer oidc-token")

	// When
	identity, ok, err := manager.AuthenticateAdmin(context.Background(), req)

	// Then
	if err != nil {
		t.Fatalf("AuthenticateAdmin() error = %v, want nil", err)
	}
	if !ok {
		t.Fatal("AuthenticateAdmin() ok = false, want true")
	}
	if !identity.Admin {
		t.Fatal("identity.Admin = false, want true")
	}
}
