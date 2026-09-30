package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wjbbeyond/guardrail/internal/llm"
	"github.com/wjbbeyond/guardrail/internal/security"
)

func (s *Server) normalize(body []byte) (llm.ChatCompletionRequest, []byte, security.Decision, error) {
	if _, err := llm.DecodeChatCompletion(body); err != nil {
		return llm.ChatCompletionRequest{}, nil, security.Decision{}, err
	}
	var fields map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&fields); err != nil {
		return llm.ChatCompletionRequest{}, nil, security.Decision{}, err
	}
	if _, ok := fields["audio"]; ok {
		return llm.ChatCompletionRequest{}, nil, security.Decision{}, fmt.Errorf("audio output is not supported")
	}
	if modalities, ok := fields["modalities"]; ok {
		values, valid := modalities.([]any)
		if !valid || len(values) != 1 || values[0] != "text" {
			return llm.ChatCompletionRequest{}, nil, security.Decision{}, fmt.Errorf("only text output is supported")
		}
	}
	limit := 1024
	field := "max_tokens"
	if _, ok := fields["max_completion_tokens"]; ok {
		if _, both := fields["max_tokens"]; both {
			return llm.ChatCompletionRequest{}, nil, security.Decision{}, fmt.Errorf("specify only one output token limit")
		}
		field = "max_completion_tokens"
	}
	if raw, ok := fields[field]; ok {
		number, valid := raw.(json.Number)
		n, err := number.Int64()
		if !valid || err != nil || n <= 0 || n > 1000000 {
			return llm.ChatCompletionRequest{}, nil, security.Decision{}, fmt.Errorf("invalid output token limit")
		}
		limit = int(n)
	}
	if n, ok := fields["n"]; ok && n != json.Number("1") {
		return llm.ChatCompletionRequest{}, nil, security.Decision{}, fmt.Errorf("only n=1 is supported")
	}
	fields[field] = limit
	var stringsToInspect []string
	var visit func(any, bool) (any, error)
	visit = func(value any, redact bool) (any, error) {
		switch v := value.(type) {
		case string:
			stringsToInspect = append(stringsToInspect, v)
			if redact {
				result, _ := s.guard.Redact(v)
				return result, nil
			}
		case []any:
			for i, child := range v {
				result, err := visit(child, redact)
				if err != nil {
					return nil, err
				}
				v[i] = result
			}
		case map[string]any:
			if kind, ok := v["type"].(string); ok && (strings.Contains(kind, "image") || strings.Contains(kind, "audio") || strings.Contains(kind, "video")) {
				return nil, fmt.Errorf("multimodal input requires a separate inspection and pricing policy")
			}
			for key, child := range v {
				// Tool arguments are JSON encoded inside a JSON string.
				if key == "arguments" {
					if text, ok := child.(string); ok {
						var arguments any
						dec := json.NewDecoder(strings.NewReader(text))
						dec.UseNumber()
						if json.Valid([]byte(text)) && dec.Decode(&arguments) == nil {
							result, err := visit(arguments, redact)
							if err != nil {
								return nil, err
							}
							encoded, err := json.Marshal(result)
							if err != nil {
								return nil, err
							}
							v[key] = string(encoded)
							continue
						}
					}
				}
				result, err := visit(child, redact)
				if err != nil {
					return nil, err
				}
				v[key] = result
			}
		}
		return value, nil
	}
	if _, err := visit(fields, false); err != nil {
		return llm.ChatCompletionRequest{}, nil, security.Decision{}, err
	}
	decision := s.guard.Inspect(strings.Join(stringsToInspect, "\n"))
	if decision.Action == security.ActionRedact {
		if _, err := visit(fields, true); err != nil {
			return llm.ChatCompletionRequest{}, nil, decision, err
		}
	}
	if fields["stream"] == true {
		fields["stream_options"] = map[string]any{"include_usage": true}
	}
	normalized, err := json.Marshal(fields)
	if err != nil {
		return llm.ChatCompletionRequest{}, nil, decision, err
	}
	chat, err := llm.DecodeChatCompletion(normalized)
	chat.MaxTokens = limit
	return chat, normalized, decision, err
}
