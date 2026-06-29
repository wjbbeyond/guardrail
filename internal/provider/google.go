package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/wjbbeyond/guardrail/internal/llm"
)

type googleRequest struct {
	Contents          []googleContent  `json:"contents"`
	SystemInstruction *googleContent   `json:"systemInstruction,omitempty"`
	GenerationConfig  googleGeneration `json:"generationConfig"`
}

type googleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []googlePart `json:"parts"`
}

type googlePart struct {
	Text string `json:"text"`
}

type googleGeneration struct {
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	Temperature     *float64 `json:"temperature,omitempty"`
}

func (p *Provider) google(ctx context.Context, chat llm.ChatCompletionRequest) (*UpstreamResponse, error) {
	if chat.Stream {
		return nil, fmt.Errorf("provider %s: google streaming adapter is not enabled in v0.1", p.Name)
	}
	body, err := json.Marshal(newGoogleRequest(chat))
	if err != nil {
		return nil, fmt.Errorf("encode google request: %w", err)
	}
	endpoint, err := p.googleEndpoint(chat.Model)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build google request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("call provider %s: %w", p.Name, err)
	}
	raw, err := readAndClose(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read google response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return &UpstreamResponse{Header: resp.Header.Clone(), Body: raw, Provider: p.Name, Status: resp.StatusCode}, nil
	}
	transformed, err := googleToOpenAI(raw, chat.Model)
	if err != nil {
		return nil, err
	}
	return &UpstreamResponse{Header: jsonHeader(), Body: transformed, Provider: p.Name, Status: resp.StatusCode}, nil
}

func (p *Provider) googleEndpoint(model string) (string, error) {
	parsed, err := url.Parse(p.Endpoint("/models/" + url.PathEscape(model) + ":generateContent"))
	if err != nil {
		return "", fmt.Errorf("build google endpoint: %w", err)
	}
	if key := p.NextKey(); key != "" {
		query := parsed.Query()
		query.Set("key", key)
		parsed.RawQuery = query.Encode()
	}
	return parsed.String(), nil
}

func newGoogleRequest(chat llm.ChatCompletionRequest) googleRequest {
	contents := make([]googleContent, 0, len(chat.Messages))
	system := make([]googlePart, 0)
	for _, message := range chat.Messages {
		switch message.Role {
		case "system", "developer":
			system = append(system, googlePart{Text: message.Text()})
		case "assistant":
			contents = append(contents, googleContent{Role: "model", Parts: []googlePart{{Text: message.Text()}}})
		default:
			contents = append(contents, googleContent{Role: "user", Parts: []googlePart{{Text: message.Text()}}})
		}
	}
	req := googleRequest{
		Contents: contents,
		GenerationConfig: googleGeneration{
			MaxOutputTokens: chat.MaxTokens,
			Temperature:     chat.Temperature,
		},
	}
	if len(system) > 0 {
		req.SystemInstruction = &googleContent{Parts: system}
	}
	return req
}

func googleToOpenAI(raw []byte, fallbackModel string) ([]byte, error) {
	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
		Usage struct {
			PromptTokens     int `json:"promptTokenCount"`
			CompletionTokens int `json:"candidatesTokenCount"`
		} `json:"usageMetadata"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return nil, fmt.Errorf("decode google response: %w", err)
	}
	texts := make([]string, 0)
	if len(res.Candidates) > 0 {
		for _, part := range res.Candidates[0].Content.Parts {
			texts = append(texts, part.Text)
		}
	}
	id := "chatcmpl-google-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	return marshalOpenAIResponse(id, fallbackModel, strings.Join(texts, ""), res.Usage.PromptTokens, res.Usage.CompletionTokens)
}
