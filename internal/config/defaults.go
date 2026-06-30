package config

import "time"

func Default() Config {
	return Config{
		Server: ServerConfig{
			ListenAddr:      ":8080",
			ReadTimeout:     15 * time.Second,
			WriteTimeout:    0,
			ShutdownTimeout: 20 * time.Second,
			LogLevel:        "info",
		},
		Auth: AuthConfig{
			Enabled: true,
			OIDC: OIDCConfig{
				TenantClaim:     "tenant",
				AdminGroupClaim: "groups",
			},
		},
		Providers: []ProviderConfig{
			{
				Name:    "openai",
				Type:    ProviderOpenAI,
				BaseURL: "https://api.openai.com/v1",
				Models:  []string{"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini"},
			},
		},
		Security: SecurityConfig{
			PromptInjectionMode: "warn",
			PIIMode:             "redact",
		},
		Cost: CostConfig{
			DailyBudgetUSD:      10,
			PerRequestBudgetUSD: 1,
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerMinute: 60,
			Burst:             10,
		},
		Pricing: PricingConfig{
			RefreshInterval: 24 * time.Hour,
		},
		Audit: AuditConfig{
			SQLiteDSN: "file:guardrail-audit.db?_pragma=busy_timeout(5000)",
		},
		Reliability: ReliabilityConfig{
			MaxRetries:      1,
			ProviderTimeout: 60 * time.Second,
		},
	}
}
