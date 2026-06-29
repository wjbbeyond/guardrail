package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/wjbbeyond/guardrail/internal/llm"
)

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (p *Provider) anthropic(ctx context.Context, chat llm.ChatCompletionRequest) (*UpstreamResponse, error) {
	if chat.Stream {
		return nil, fmt.Errorf("provider %s: anthropic streaming adapter is not enabled in v0.1", p.Name)
	}
	body, err := json.Marshal(newAnthropicRequest(chat))
	if err != nil {
		return nil, fmt.Errorf("encode anthropic request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.Endpoint("/messages"), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Anthropic-Version", "2023-06-01")
	if key := p.NextKey(); key != "" {
		req.Header.Set("X-API-Key", key)
	}
	resp, err := p.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("call provider %s: %w", p.Name, err)
	}
	raw, err := readAndClose(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anthropic response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return &UpstreamResponse{Header: resp.Header.Clone(), Body: raw, Provider: p.Name, Status: resp.StatusCode}, nil
	}
	transformed, err := anthropicToOpenAI(raw, chat.Model)
	if err != nil {
		return nil, err
	}
	return &UpstreamResponse{Header: jsonHeader(), Body: transformed, Provider: p.Name, Status: resp.StatusCode}, nil
}

func newAnthropicRequest(chat llm.ChatCompletionRequest) anthropicRequest {
	messages := make([]anthropicMessage, 0, len(chat.Messages))
	system := make([]string, 0)
	for _, message := range chat.Messages {
		switch message.Role {
		case "system", "developer":
			system = append(system, message.Text())
		case "assistant":
			messages = append(messages, anthropicMessage{Role: "assistant", Content: message.Text()})
		default:
			messages = append(messages, anthropicMessage{Role: "user", Content: message.Text()})
		}
	}
	maxTokens := chat.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	return anthropicRequest{
		Model:     chat.Model,
		MaxTokens: maxTokens,
		System:    strings.Join(system, "\n"),
		Messages:  messages,
	}
}

func anthropicToOpenAI(raw []byte, fallbackModel string) ([]byte, error) {
	var res struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}
	texts := make([]string, 0, len(res.Content))
	for _, part := range res.Content {
		texts = append(texts, part.Text)
	}
	model := res.Model
	if model == "" {
		model = fallbackModel
	}
	return marshalOpenAIResponse(res.ID, model, strings.Join(texts, ""), res.Usage.InputTokens, res.Usage.OutputTokens)
}
