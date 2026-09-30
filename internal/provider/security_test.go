package provider

import (
	"context"
	"fmt"
	"github.com/wjbbeyond/guardrail/internal/config"
	"github.com/wjbbeyond/guardrail/internal/llm"
	"net/http"
	"strings"
	"testing"
	"time"
)

type failingTransport struct{ t *testing.T }

func (f failingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Query().Get("key") != "" || r.Header.Get("x-goog-api-key") != "fake-secret" {
		f.t.Fatal("unsafe auth transport")
	}
	return nil, fmt.Errorf("simulated network failure")
}
func TestGoogleKeyNotInTransportError(t *testing.T) {
	router, err := NewRouter([]config.ProviderConfig{{Name: "google", Type: config.ProviderGoogle, BaseURL: "https://example.invalid", APIKeys: []string{"fake-secret"}}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	p := router.providers[0]
	p.client.Transport = failingTransport{t}
	_, err = p.google(context.Background(), llm.ChatCompletionRequest{Model: "fake"})
	if err == nil || strings.Contains(err.Error(), "fake-secret") {
		t.Fatalf("unsafe error: %v", err)
	}
}
