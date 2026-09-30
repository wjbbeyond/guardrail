package gateway

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/wjbbeyond/guardrail/internal/cost"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEscapedPIIAndOutputLimit(t *testing.T) {
	server, _ := newTestServer(t, context.Background(), "http://example.invalid", 10)
	chat, body, _, err := server.normalize([]byte(`{"model":"gpt-4o","messages":[{"role":"user","content":[{"type":"text","text":"alice\u0040example.com"}]}],"tools":[{"description":"bob\u0040example.com"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if chat.MaxTokens != 1024 || strings.Contains(string(body), "example.com") || !strings.Contains(string(body), `"max_tokens":1024`) {
		t.Fatalf("unsafe request: %s", body)
	}
	for _, invalid := range []string{"null", "{}", "[]"} {
		if _, _, _, err := server.normalize([]byte(invalid)); err == nil {
			t.Fatal("accepted invalid payload", invalid)
		}
	}
	_, nested, _, err := server.normalize([]byte(`{"model":"gpt-4o","messages":[{"role":"assistant","tool_calls":[{"function":{"arguments":"{\"email\":\"alice\\u0040example.com\"}"}}]}]}`))
	if err != nil || strings.Contains(string(nested), "example.com") {
		t.Fatalf("nested tool PII: %s %v", nested, err)
	}
	for _, extra := range []string{`"modalities":["audio"]`, `"audio":{"format":"wav"}`, `"n":2`, `"max_tokens":-1`, `"max_tokens":1,"max_completion_tokens":2`} {
		_, _, _, err = server.normalize([]byte(`{"model":"gpt-4o","messages":[{"content":"hello"}],` + extra + `}`))
		if err == nil {
			t.Fatal("accepted", extra)
		}
	}
}
func TestStreamUsageAndCancellationPersist(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "cancelled"}[cancelled], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n")
				w.(http.Flusher).Flush()
				if cancelled {
					cancel()
					return
				}
				io.WriteString(w, "data: {\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":8}}\n\ndata: [DONE]\n\n")
			}))
			defer upstream.Close()
			server, audit := newTestServer(t, context.Background(), upstream.URL, 10)
			ledger, err := cost.OpenSQLiteLedger(context.Background(), "file:"+t.TempDir()+"/cost.db")
			if err != nil {
				t.Fatal(err)
			}
			defer ledger.Close()
			server.costs = cost.NewTrackerWithLedger(server.cfg.Cost, cost.RealClock{}, ledger)
			request := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","stream":true,"messages":[{"role":"user","content":"hello"}],"max_tokens":100}`)).WithContext(ctx)
			server.Handler().ServeHTTP(httptest.NewRecorder(), request)
			snapshot, err := server.costs.Snapshot(context.Background())
			if err != nil || snapshot.SpentUSD <= 0 {
				t.Fatalf("cost lost: %+v %v", snapshot, err)
			}
			events, err := audit.Recent(context.Background(), 10)
			if err != nil || len(events) != 1 {
				t.Fatalf("audit lost: %+v %v", events, err)
			}
			if !cancelled && events[0].CompletionTokens != 8 {
				t.Fatalf("usage lost: %+v", events[0])
			}
		})
	}
}

func TestStreamFlushesBeforeUpstreamCompletes(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"choices\":[]}\n\n")
		w.(http.Flusher).Flush()
		<-release
	}))
	defer upstream.Close()
	defer close(release)
	server, _ := newTestServer(t, context.Background(), upstream.URL, 10)
	gateway := httptest.NewServer(server.Handler())
	defer gateway.Close()
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Post(gateway.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{"model":"gpt-4o","stream":true,"messages":[{"content":"test"}]}`))
	if err != nil {
		t.Fatal("first frame buffered until completion:", err)
	}
	defer resp.Body.Close()
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "data:") {
		t.Fatalf("first frame: %q %v", line, err)
	}
}

type errorTransport struct{}

func (errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("fake-provider-secret")
}

func TestTransportErrorsHideSecretsAndAuditReservedCost(t *testing.T) {
	server, audit := newTestServer(t, context.Background(), "http://example.invalid", 10)
	server.router.Candidates("gpt-4o")[0].Client().Transport = errorTransport{}
	var logs bytes.Buffer
	server.logger = slog.New(slog.NewTextHandler(&logs, nil))
	request := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o","messages":[{"content":"test"}]}`))
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != 502 || strings.Contains(response.Body.String()+logs.String(), "fake-provider-secret") {
		t.Fatalf("unsafe error response/log: %s %s", response.Body.String(), logs.String())
	}
	events, err := audit.Recent(context.Background(), 1)
	if err != nil || len(events) != 1 || events[0].CostUSD <= 0 {
		t.Fatalf("reserved charge missing from audit: %+v %v", events, err)
	}
}
