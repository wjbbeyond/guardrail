package cost

import "encoding/json"

func EstimateTokens(text string) int {
	runes := len([]rune(text))
	if runes == 0 {
		return 0
	}
	return (runes + 3) / 4
}

func CompletionTokensFromOpenAI(body []byte) int {
	var payload struct {
		Usage struct {
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return EstimateTokens(string(body))
	}
	if payload.Usage.CompletionTokens > 0 {
		return payload.Usage.CompletionTokens
	}
	return EstimateTokens(string(body))
}
