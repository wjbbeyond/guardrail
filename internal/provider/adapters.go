package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/wjbbeyond/guardrail/internal/config"
	"github.com/wjbbeyond/guardrail/internal/llm"
)

type UpstreamResponse struct {
	Header    http.Header
	Body      []byte
	Stream    io.ReadCloser
	Provider  string
	Status    int
	Streaming bool
}

func (p *Provider) ChatCompletions(ctx context.Context, chat llm.ChatCompletionRequest, rawBody []byte) (*UpstreamResponse, error) {
	switch p.Type {
	case config.ProviderOpenAI, config.ProviderOpenAICompatible:
		return p.openAI(ctx, chat, rawBody)
	case config.ProviderAnthropic:
		return p.anthropic(ctx, chat)
	case config.ProviderGoogle:
		return p.google(ctx, chat)
	default:
		return nil, fmt.Errorf("provider %s has unsupported type %q", p.Name, p.Type)
	}
}

func readAndClose(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func jsonHeader() http.Header {
	header := make(http.Header)
	header.Set("Content-Type", "application/json")
	return header
}
