package config

import "time"

type ProviderType string

const (
	ProviderOpenAICompatible ProviderType = "openai-compatible"
	ProviderOpenAI           ProviderType = "openai"
	ProviderAnthropic        ProviderType = "anthropic"
	ProviderGoogle           ProviderType = "google"
)

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Auth        AuthConfig        `yaml:"auth"`
	Tenants     []TenantConfig    `yaml:"tenants"`
	Providers   []ProviderConfig  `yaml:"providers"`
	Security    SecurityConfig    `yaml:"security"`
	Cost        CostConfig        `yaml:"cost"`
	RateLimit   RateLimitConfig   `yaml:"rate_limit"`
	Pricing     PricingConfig     `yaml:"pricing"`
	Audit       AuditConfig       `yaml:"audit"`
	Reliability ReliabilityConfig `yaml:"reliability"`
}

type ServerConfig struct {
	ListenAddr      string        `yaml:"listen_addr"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	WriteTimeout    time.Duration `yaml:"write_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	LogLevel        string        `yaml:"log_level"`
}

type AuthConfig struct {
	Enabled      bool       `yaml:"enabled"`
	ProxyAPIKeys []string   `yaml:"proxy_api_keys"`
	AdminAPIKeys []string   `yaml:"admin_api_keys"`
	OIDC         OIDCConfig `yaml:"oidc"`
}

type OIDCConfig struct {
	Enabled         bool     `yaml:"enabled"`
	IssuerURL       string   `yaml:"issuer_url"`
	ClientID        string   `yaml:"client_id"`
	TenantClaim     string   `yaml:"tenant_claim"`
	AdminGroupClaim string   `yaml:"admin_group_claim"`
	AdminGroups     []string `yaml:"admin_groups"`
}

type TenantConfig struct {
	ID                  string          `yaml:"id"`
	ProxyAPIKeys        []string        `yaml:"proxy_api_keys"`
	DailyBudgetUSD      float64         `yaml:"daily_budget_usd"`
	PerRequestBudgetUSD float64         `yaml:"per_request_budget_usd"`
	RateLimit           RateLimitConfig `yaml:"rate_limit"`
}

type ProviderConfig struct {
	Name    string       `yaml:"name"`
	Type    ProviderType `yaml:"type"`
	BaseURL string       `yaml:"base_url"`
	APIKeys []string     `yaml:"api_keys"`
	Models  []string     `yaml:"models"`
}

type SecurityConfig struct {
	PromptInjectionMode string   `yaml:"prompt_injection_mode"`
	PIIMode             string   `yaml:"pii_mode"`
	ExtraPIIPatterns    []string `yaml:"extra_pii_patterns"`
}

type CostConfig struct {
	DailyBudgetUSD      float64 `yaml:"daily_budget_usd"`
	PerRequestBudgetUSD float64 `yaml:"per_request_budget_usd"`
}

type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
	Burst             int  `yaml:"burst"`
}

type PricingConfig struct {
	URL             string        `yaml:"url"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

type AuditConfig struct {
	SQLiteDSN string `yaml:"sqlite_dsn"`
}

type ReliabilityConfig struct {
	MaxRetries      int           `yaml:"max_retries"`
	ProviderTimeout time.Duration `yaml:"provider_timeout"`
}
