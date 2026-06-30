package authn

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/wjbbeyond/guardrail/internal/config"
)

const (
	ProxyAPIKeyHeader = "X-GuardRail-API-Key"
	AdminAPIKeyHeader = "X-GuardRail-Admin-Key"
)

type Manager struct {
	proxyKeys       map[[32]byte]string
	adminKeys       [][32]byte
	knownTenants    map[string]struct{}
	oidc            TokenVerifier
	tenantClaim     string
	adminGroupClaim string
	adminGroups     []string
}

type Token struct {
	Subject string
	Claims  map[string]any
}

type TokenVerifier interface {
	Verify(ctx context.Context, rawToken string) (Token, error)
}

func NewManager(ctx context.Context, cfg config.Config) (*Manager, error) {
	var verifier TokenVerifier
	if cfg.Auth.OIDC.Enabled {
		oidcVerifier, err := NewOIDCVerifier(ctx, cfg.Auth.OIDC)
		if err != nil {
			return nil, fmt.Errorf("build oidc verifier: %w", err)
		}
		verifier = oidcVerifier
	}
	return NewManagerWithVerifier(cfg, verifier), nil
}

func NewManagerWithVerifier(cfg config.Config, verifier TokenVerifier) *Manager {
	manager := &Manager{
		proxyKeys:       make(map[[32]byte]string),
		knownTenants:    make(map[string]struct{}),
		oidc:            verifier,
		tenantClaim:     claimOrDefault(cfg.Auth.OIDC.TenantClaim, "tenant"),
		adminGroupClaim: claimOrDefault(cfg.Auth.OIDC.AdminGroupClaim, "groups"),
		adminGroups:     nonEmptyStrings(cfg.Auth.OIDC.AdminGroups),
	}
	for _, key := range cfg.Auth.ProxyAPIKeys {
		manager.addProxyKey(key, DefaultTenantID)
	}
	for _, key := range cfg.Auth.AdminAPIKeys {
		manager.addAdminKey(key)
	}
	for _, tenant := range cfg.Tenants {
		tenantID := strings.TrimSpace(tenant.ID)
		if tenantID == "" {
			continue
		}
		manager.knownTenants[tenantID] = struct{}{}
		for _, key := range tenant.ProxyAPIKeys {
			manager.addProxyKey(key, tenantID)
		}
	}
	return manager
}

func (m *Manager) AuthenticateProxy(ctx context.Context, r *http.Request) (Identity, bool, error) {
	if tenantID, ok := m.staticProxyTenant(r); ok {
		return Identity{TenantID: tenantID, Subject: tenantID, Method: "api_key"}, true, nil
	}
	rawToken := bearerToken(r.Header.Get("Authorization"))
	if rawToken == "" || m.oidc == nil {
		return Identity{}, false, nil
	}
	identity, err := m.identityFromOIDC(ctx, rawToken)
	if err != nil {
		return Identity{}, false, err
	}
	return identity, true, nil
}

func (m *Manager) AuthenticateAdmin(ctx context.Context, r *http.Request) (Identity, bool, error) {
	if m.hasAdminKey(r) {
		return Identity{TenantID: DefaultTenantID, Subject: "admin", Method: "admin_api_key", Admin: true}, true, nil
	}
	rawToken := bearerToken(r.Header.Get("Authorization"))
	if rawToken == "" || m.oidc == nil || len(m.adminGroups) == 0 {
		return Identity{}, false, nil
	}
	token, err := m.oidc.Verify(ctx, rawToken)
	if err != nil {
		return Identity{}, false, fmt.Errorf("verify oidc admin token: %w", err)
	}
	if !hasAnyStringClaim(token.Claims[m.adminGroupClaim], m.adminGroups) {
		return Identity{}, false, nil
	}
	return Identity{TenantID: DefaultTenantID, Subject: token.Subject, Method: "oidc", Admin: true}, true, nil
}

func (m *Manager) KnownTenant(tenantID string) bool {
	if strings.TrimSpace(tenantID) == DefaultTenantID {
		return true
	}
	if len(m.knownTenants) == 0 {
		return true
	}
	_, ok := m.knownTenants[tenantID]
	return ok
}

func (m *Manager) addProxyKey(key string, tenantID string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	m.proxyKeys[sha256.Sum256([]byte(key))] = tenantID
}

func (m *Manager) addAdminKey(key string) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}
	m.adminKeys = append(m.adminKeys, sha256.Sum256([]byte(key)))
}

func (m *Manager) staticProxyTenant(r *http.Request) (string, bool) {
	for _, presented := range presentedKeys(r, ProxyAPIKeyHeader) {
		hash := sha256.Sum256([]byte(presented))
		for keyHash, tenantID := range m.proxyKeys {
			if subtle.ConstantTimeCompare(hash[:], keyHash[:]) == 1 {
				return tenantID, true
			}
		}
	}
	return "", false
}

func (m *Manager) hasAdminKey(r *http.Request) bool {
	for _, presented := range presentedKeys(r, AdminAPIKeyHeader) {
		hash := sha256.Sum256([]byte(presented))
		for _, keyHash := range m.adminKeys {
			if subtle.ConstantTimeCompare(hash[:], keyHash[:]) == 1 {
				return true
			}
		}
	}
	return false
}

func (m *Manager) identityFromOIDC(ctx context.Context, rawToken string) (Identity, error) {
	token, err := m.oidc.Verify(ctx, rawToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verify oidc token: %w", err)
	}
	tenantID := stringClaim(token.Claims[m.tenantClaim])
	if tenantID == "" {
		if len(m.knownTenants) > 0 {
			return Identity{}, fmt.Errorf("missing tenant claim %q", m.tenantClaim)
		}
		tenantID = DefaultTenantID
	}
	if !m.KnownTenant(tenantID) {
		return Identity{}, fmt.Errorf("unknown tenant %q", tenantID)
	}
	return Identity{TenantID: tenantID, Subject: token.Subject, Method: "oidc"}, nil
}

func presentedKeys(r *http.Request, header string) []string {
	keys := make([]string, 0, 2)
	if key := strings.TrimSpace(r.Header.Get(header)); key != "" {
		keys = append(keys, key)
	}
	if key := bearerToken(r.Header.Get("Authorization")); key != "" {
		keys = append(keys, key)
	}
	return keys
}

func bearerToken(value string) string {
	value = strings.TrimSpace(value)
	token, ok := strings.CutPrefix(value, "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(token)
}

func claimOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func stringClaim(value any) string {
	raw, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(raw)
}

func hasAnyStringClaim(value any, allowed []string) bool {
	switch typed := value.(type) {
	case string:
		return slices.Contains(allowed, typed)
	case []any:
		for _, item := range typed {
			if raw, ok := item.(string); ok && slices.Contains(allowed, raw) {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if slices.Contains(allowed, item) {
				return true
			}
		}
	}
	return false
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
