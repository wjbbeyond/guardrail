package llm

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type ChatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type ContentPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func DecodeChatCompletion(body []byte) (ChatCompletionRequest, error) {
	var req ChatCompletionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return ChatCompletionRequest{}, fmt.Errorf("decode chat completion request: %w", err)
	}
	if strings.TrimSpace(req.Model) == "" {
		return ChatCompletionRequest{}, fmt.Errorf("chat completion request: model is required")
	}
	if len(req.Messages) == 0 {
		return ChatCompletionRequest{}, fmt.Errorf("chat completion request: messages are required")
	}
	return req, nil
}

func (r ChatCompletionRequest) PromptText() string {
	parts := make([]string, 0, len(r.Messages))
	for _, msg := range r.Messages {
		text := msg.Text()
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func (m ChatMessage) Text() string {
	var text string
	if err := json.Unmarshal(m.Content, &text); err == nil {
		return text
	}

	var parts []ContentPart
	if err := json.Unmarshal(m.Content, &parts); err != nil {
		return ""
	}
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Text != "" {
			texts = append(texts, part.Text)
		}
	}
	return strings.Join(texts, "\n")
}
