package config

import (
	"os"
	"strings"
)

func applyEnv(cfg *Config) {
	if listen := os.Getenv("GUARDRAIL_LISTEN_ADDR"); listen != "" {
		cfg.Server.ListenAddr = listen
	}
	if dsn := os.Getenv("GUARDRAIL_AUDIT_SQLITE_DSN"); dsn != "" {
		cfg.Audit.SQLiteDSN = dsn
	}
	appendEnvKeys(&cfg.Auth.ProxyAPIKeys, os.Getenv("GUARDRAIL_PROXY_API_KEY"))
	appendEnvKeys(&cfg.Auth.ProxyAPIKeys, os.Getenv("GUARDRAIL_PROXY_API_KEYS"))
	appendEnvKeys(&cfg.Auth.AdminAPIKeys, os.Getenv("GUARDRAIL_ADMIN_API_KEY"))
	appendEnvKeys(&cfg.Auth.AdminAPIKeys, os.Getenv("GUARDRAIL_ADMIN_API_KEYS"))
	if issuer := os.Getenv("GUARDRAIL_OIDC_ISSUER_URL"); issuer != "" {
		cfg.Auth.OIDC.Enabled = true
		cfg.Auth.OIDC.IssuerURL = issuer
	}
	if clientID := os.Getenv("GUARDRAIL_OIDC_CLIENT_ID"); clientID != "" {
		cfg.Auth.OIDC.Enabled = true
		cfg.Auth.OIDC.ClientID = clientID
	}
	if claim := os.Getenv("GUARDRAIL_OIDC_TENANT_CLAIM"); claim != "" {
		cfg.Auth.OIDC.TenantClaim = claim
	}
	applyProviderKey(cfg, "openai", os.Getenv("OPENAI_API_KEY"))
	applyProviderKey(cfg, "anthropic", os.Getenv("ANTHROPIC_API_KEY"))
	applyProviderKey(cfg, "google", os.Getenv("GEMINI_API_KEY"))
}

func appendEnvKeys(dst *[]string, raw string) {
	if raw == "" {
		return
	}
	for _, key := range strings.Split(raw, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			*dst = append(*dst, key)
		}
	}
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

func applyProviderKey(cfg *Config, name string, key string) {
	if key == "" {
		return
	}
	for i := range cfg.Providers {
		if cfg.Providers[i].Name == name && len(cfg.Providers[i].APIKeys) == 0 {
			cfg.Providers[i].APIKeys = []string{key}
			return
		}
	}
}
