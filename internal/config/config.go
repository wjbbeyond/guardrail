package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

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
	Providers   []ProviderConfig  `yaml:"providers"`
	Security    SecurityConfig    `yaml:"security"`
	Cost        CostConfig        `yaml:"cost"`
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
	Enabled      bool     `yaml:"enabled"`
	ProxyAPIKeys []string `yaml:"proxy_api_keys"`
	AdminAPIKeys []string `yaml:"admin_api_keys"`
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

type AuditConfig struct {
	SQLiteDSN string `yaml:"sqlite_dsn"`
}

type ReliabilityConfig struct {
	MaxRetries      int           `yaml:"max_retries"`
	ProviderTimeout time.Duration `yaml:"provider_timeout"`
}

func Load(path string) (Config, error) {
	cfg := Default()
	configPath := path
	if configPath == "" {
		configPath = os.Getenv("GUARDRAIL_CONFIG")
	}
	if configPath == "" {
		configPath = "configs/guardrail.yaml"
	}

	if raw, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(raw, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode %s: %w", configPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read %s: %w", configPath, err)
	}

	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

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
		Audit: AuditConfig{
			SQLiteDSN: "file:guardrail-audit.db?_pragma=busy_timeout(5000)",
		},
		Reliability: ReliabilityConfig{
			MaxRetries:      1,
			ProviderTimeout: 60 * time.Second,
		},
	}
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Server.ListenAddr) == "" {
		return errors.New("config: server.listen_addr is required")
	}
	if len(c.Providers) == 0 {
		return errors.New("config: at least one provider is required")
	}
	if err := validateAuth(c.Auth); err != nil {
		return err
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

func validateAuth(auth AuthConfig) error {
	if !auth.Enabled {
		return nil
	}
	if len(nonEmptyStrings(auth.ProxyAPIKeys)) == 0 {
		return errors.New("config: auth.proxy_api_keys is required when auth.enabled is true")
	}
	if len(nonEmptyStrings(auth.AdminAPIKeys)) == 0 {
		return errors.New("config: auth.admin_api_keys is required when auth.enabled is true")
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
