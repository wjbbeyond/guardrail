package provider

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/wjbbeyond/guardrail/internal/llm"
)

func (p *Provider) openAI(ctx context.Context, chat llm.ChatCompletionRequest, rawBody []byte) (*UpstreamResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint("/chat/completions"), bytes.NewReader(rawBody))
	if err != nil {
		return nil, fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if key := p.NextKey(); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := p.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("call provider %s: %w", p.Name, err)
	}
	if chat.Stream && resp.StatusCode >= 200 && resp.StatusCode < 300 && strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return &UpstreamResponse{
			Header:    resp.Header.Clone(),
			Stream:    resp.Body,
			Provider:  p.Name,
			Status:    resp.StatusCode,
			Streaming: true,
		}, nil
	}
	body, err := readAndClose(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read provider %s response: %w", p.Name, err)
	}
	return &UpstreamResponse{Header: resp.Header.Clone(), Body: body, Provider: p.Name, Status: resp.StatusCode}, nil
}
