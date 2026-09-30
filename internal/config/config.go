package config

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

func Load(path string) (Config, error) {
	cfg := Default()
	explicit := path != "" || os.Getenv("GUARDRAIL_CONFIG") != ""
	configPath := path
	if configPath == "" {
		configPath = os.Getenv("GUARDRAIL_CONFIG")
	}
	if configPath == "" {
		configPath = "configs/guardrail.yaml"
	}

	if raw, err := os.ReadFile(configPath); err == nil {
		decoder := yaml.NewDecoder(bytes.NewReader(raw))
		decoder.KnownFields(true)
		if err := decoder.Decode(&cfg); err != nil {
			return Config{}, fmt.Errorf("decode %s: %w", configPath, err)
		}
	} else if explicit || !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read %s: %w", configPath, err)
	}

	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	for _, mode := range []string{c.Security.PIIMode, c.Security.PromptInjectionMode} {
		switch strings.ToLower(strings.TrimSpace(mode)) {
		case "off", "warn", "redact", "block":
		default:
			return errors.New("config: invalid security mode")
		}
	}
	for _, pattern := range c.Security.ExtraPIIPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("config: invalid PII pattern: %w", err)
		}
	}
	budgets := []float64{c.Cost.DailyBudgetUSD, c.Cost.PerRequestBudgetUSD}
	keys := map[string]bool{}
	for _, key := range append(append([]string{}, c.Auth.AdminAPIKeys...), c.Auth.ProxyAPIKeys...) {
		if strings.TrimSpace(key) != "" {
			if keys[key] {
				return errors.New("config: duplicate authentication key")
			}
			keys[key] = true
		}
	}
	for _, tenant := range c.Tenants {
		budgets = append(budgets, tenant.DailyBudgetUSD, tenant.PerRequestBudgetUSD)
		if tenant.ID != strings.TrimSpace(tenant.ID) {
			return errors.New("config: tenant id must not have surrounding whitespace")
		}
		for _, key := range tenant.ProxyAPIKeys {
			if strings.TrimSpace(key) != "" {
				if keys[key] {
					return errors.New("config: duplicate authentication key")
				}
				keys[key] = true
			}
		}
	}
	for _, value := range budgets {
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
			return errors.New("config: budgets must be finite and nonnegative")
		}
	}

	if strings.TrimSpace(c.Server.ListenAddr) == "" {
		return errors.New("config: server.listen_addr is required")
	}
	if len(c.Providers) == 0 {
		return errors.New("config: at least one provider is required")
	}
	if err := validateTenants(c.Tenants); err != nil {
		return err
	}
	if err := validateAuth(c.Auth, c.Tenants); err != nil {
		return err
	}
	if err := validateRateLimit(c.RateLimit, "rate_limit"); err != nil {
		return err
	}
	if c.Pricing.RefreshInterval < 0 {
		return errors.New("config: pricing.refresh_interval must not be negative")
	}
	for _, provider := range c.Providers {
		if err := validateProvider(provider); err != nil {
			return err
		}
	}
	if c.Audit.SQLiteDSN == "" {
		return errors.New("config: audit.sqlite_dsn is required")
	}
	if c.Reliability.ProviderTimeout <= 0 {
		return errors.New("config: reliability.provider_timeout must be positive")
	}
	return nil
}

func validateAuth(auth AuthConfig, tenants []TenantConfig) error {
	if !auth.Enabled {
		return nil
	}
	if len(nonEmptyStrings(auth.AdminAPIKeys)) == 0 {
		return errors.New("config: auth.admin_api_keys is required when auth.enabled is true")
	}
	if auth.OIDC.Enabled {
		if strings.TrimSpace(auth.OIDC.IssuerURL) == "" {
			return errors.New("config: auth.oidc.issuer_url is required when auth.oidc.enabled is true")
		}
		if strings.TrimSpace(auth.OIDC.ClientID) == "" {
			return errors.New("config: auth.oidc.client_id is required when auth.oidc.enabled is true")
		}
	}
	if len(nonEmptyStrings(auth.ProxyAPIKeys)) == 0 && !hasTenantProxyKeys(tenants) && !auth.OIDC.Enabled {
		return errors.New("config: auth.proxy_api_keys or auth.oidc is required when auth.enabled is true")
	}
	return nil
}

func hasTenantProxyKeys(tenants []TenantConfig) bool {
	for _, tenant := range tenants {
		if len(nonEmptyStrings(tenant.ProxyAPIKeys)) > 0 {
			return true
		}
	}
	return false
}

func validateTenants(tenants []TenantConfig) error {
	seen := make(map[string]struct{}, len(tenants))
	for _, tenant := range tenants {
		id := strings.TrimSpace(tenant.ID)
		if id == "" {
			return errors.New("config: tenant.id is required")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("config: duplicate tenant id %q", id)
		}
		seen[id] = struct{}{}
		if err := validateRateLimit(tenant.RateLimit, "tenant "+id+" rate_limit"); err != nil {
			return err
		}
	}
	return nil
}

func validateRateLimit(rate RateLimitConfig, path string) error {
	if !rate.Enabled && rate.RequestsPerMinute == 0 && rate.Burst == 0 {
		return nil
	}
	if rate.RequestsPerMinute < 0 || rate.Burst < 0 {
		return fmt.Errorf("config: %s values must not be negative", path)
	}
	if rate.Enabled && rate.RequestsPerMinute == 0 {
		return fmt.Errorf("config: %s.requests_per_minute is required when enabled", path)
	}
	if rate.Enabled && rate.Burst == 0 {
		return fmt.Errorf("config: %s.burst is required when enabled", path)
	}
	return nil
}

func validateProvider(provider ProviderConfig) error {
	if strings.TrimSpace(provider.Name) == "" {
		return errors.New("config: provider.name is required")
	}
	switch provider.Type {
	case ProviderOpenAICompatible, ProviderOpenAI, ProviderAnthropic, ProviderGoogle:
	default:
		return fmt.Errorf("config: provider %s has unsupported type %q", provider.Name, provider.Type)
	}
	parsed, err := url.Parse(provider.BaseURL)
	if err != nil {
		return fmt.Errorf("config: provider %s base_url: %w", provider.Name, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("config: provider %s base_url must be absolute", provider.Name)
	}
	return nil
}
