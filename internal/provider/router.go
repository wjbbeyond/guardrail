package provider

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wjbbeyond/guardrail/internal/config"
)

type Provider struct {
	baseURL *url.URL
	client  *http.Client
	keys    []string
	models  map[string]struct{}
	Name    string
	Type    config.ProviderType
	nextKey atomic.Uint64
}

type Router struct {
	providers []*Provider
}

func NewRouter(configs []config.ProviderConfig, timeout time.Duration) (*Router, error) {
	providers := make([]*Provider, 0, len(configs))
	for _, cfg := range configs {
		parsed, err := url.Parse(cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("parse provider %s base url: %w", cfg.Name, err)
		}
		models := make(map[string]struct{}, len(cfg.Models))
		for _, model := range cfg.Models {
			models[model] = struct{}{}
		}
		providers = append(providers, &Provider{
			Name:    cfg.Name,
			Type:    cfg.Type,
			baseURL: parsed,
			keys:    append([]string(nil), cfg.APIKeys...),
			models:  models,
			client:  &http.Client{Timeout: timeout},
		})
	}
	return &Router{providers: providers}, nil
}

func (r *Router) Candidates(model string) []*Provider {
	matches := make([]*Provider, 0, len(r.providers))
	fallback := make([]*Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		if len(provider.models) == 0 {
			fallback = append(fallback, provider)
			continue
		}
		if _, ok := provider.models[model]; ok {
			matches = append(matches, provider)
		}
	}
	if len(matches) > 0 {
		return matches
	}
	return fallback
}

func (p *Provider) Endpoint(path string) string {
	return strings.TrimRight(p.baseURL.String(), "/") + path
}

func (p *Provider) NextKey() string {
	if len(p.keys) == 0 {
		return ""
	}
	idx := p.nextKey.Add(1)
	return p.keys[(int(idx)-1)%len(p.keys)]
}

func (p *Provider) Client() *http.Client {
	return p.client
}
