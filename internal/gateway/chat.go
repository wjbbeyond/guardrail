package gateway

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/wjbbeyond/guardrail/internal/cost"
	"github.com/wjbbeyond/guardrail/internal/llm"
	"github.com/wjbbeyond/guardrail/internal/provider"
	"github.com/wjbbeyond/guardrail/internal/security"
)

const maxRequestBytes = 8 << 20

func (s *Server) chatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.metrics.RecordRequest()

	body, err := readRequestBody(r)
	if err != nil {
		s.metrics.RecordBlocked()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	chat, err := llm.DecodeChatCompletion(body)
	if err != nil {
		s.metrics.RecordBlocked()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	decision := s.guard.Inspect(chat.PromptText())
	if decision.Action == security.ActionBlock {
		s.metrics.RecordBlocked()
		w.Header().Set("X-GuardRail-Security", securityHeader(decision))
		writeError(w, http.StatusForbidden, "request blocked by GuardRail security policy")
		s.recordAudit(r.Context(), auditInput{start: start, route: r.URL.Path, model: chat.Model, status: http.StatusForbidden, action: decision.Action})
		return
	}

	forwardBody := body
	if decision.Action == security.ActionRedact {
		redacted, _ := s.guard.Redact(string(body))
		forwardBody = []byte(redacted)
		chat, err = llm.DecodeChatCompletion(forwardBody)
		if err != nil {
			s.metrics.RecordBlocked()
			writeError(w, http.StatusBadRequest, "redacted request is not valid JSON")
			return
		}
	}

	promptTokens := cost.EstimateTokens(chat.PromptText())
	maxTokens := chat.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1024
	}
	budget := s.costs.Allow(chat.Model, promptTokens, maxTokens)
	if !budget.Allowed {
		s.metrics.RecordBlocked()
		writeJSON(w, http.StatusPaymentRequired, budget)
		s.recordAudit(r.Context(), auditInput{start: start, route: r.URL.Path, model: chat.Model, status: http.StatusPaymentRequired, action: security.ActionBlock, promptTokens: promptTokens})
		return
	}

	upstream, err := s.callProviders(r.Context(), chat, forwardBody)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "all providers failed", slog.Any("err", err))
		writeError(w, http.StatusBadGateway, err.Error())
		s.recordAudit(r.Context(), auditInput{start: start, route: r.URL.Path, model: chat.Model, status: http.StatusBadGateway, action: decision.Action, promptTokens: promptTokens})
		return
	}

	w.Header().Set("X-GuardRail-Security", securityHeader(decision))
	if upstream.Streaming {
		s.writeStream(w, r, upstream)
		usage := s.costs.Record(chat.Model, promptTokens, 0)
		s.metrics.RecordCost(usage.CostUSD)
		s.recordAudit(r.Context(), auditInput{start: start, route: r.URL.Path, provider: upstream.Provider, model: chat.Model, status: upstream.Status, action: decision.Action, usage: usage})
		return
	}

	copyHeaders(w.Header(), upstream.Header)
	w.WriteHeader(upstream.Status)
	if _, err := w.Write(upstream.Body); err != nil {
		s.logger.ErrorContext(r.Context(), "write provider response", slog.Any("err", err))
	}
	completionTokens := cost.CompletionTokensFromOpenAI(upstream.Body)
	usage := s.costs.Record(chat.Model, promptTokens, completionTokens)
	s.metrics.RecordCost(usage.CostUSD)
	usage.PromptTokens = promptTokens
	s.recordAudit(r.Context(), auditInput{start: start, route: r.URL.Path, provider: upstream.Provider, model: chat.Model, status: upstream.Status, action: decision.Action, usage: usage})
}

func readRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	limited := http.MaxBytesReader(nil, r.Body, maxRequestBytes)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	return body, nil
}

func (s *Server) callProviders(ctx context.Context, chat llm.ChatCompletionRequest, body []byte) (*provider.UpstreamResponse, error) {
	candidates := s.router.Candidates(chat.Model)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no provider configured for model %s", chat.Model)
	}

	var lastErr error
	for i, candidate := range candidates {
		if i > 0 {
			s.metrics.RecordFailover()
		}
		upstream, err := candidate.ChatCompletions(ctx, chat, body)
		if err != nil {
			lastErr = err
			continue
		}
		if upstream.Status == http.StatusTooManyRequests || upstream.Status >= http.StatusInternalServerError {
			lastErr = fmt.Errorf("provider %s returned %d", candidate.Name, upstream.Status)
			continue
		}
		return upstream, nil
	}
	return nil, lastErr
}

func (s *Server) writeStream(w http.ResponseWriter, r *http.Request, upstream *provider.UpstreamResponse) {
	defer upstream.Stream.Close()
	copyHeaders(w.Header(), upstream.Header)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(upstream.Status)

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.logger.ErrorContext(r.Context(), "response writer does not support streaming")
		return
	}
	if _, err := io.Copy(w, upstream.Stream); err != nil {
		s.logger.ErrorContext(r.Context(), "copy stream", slog.Any("err", err))
		return
	}
	flusher.Flush()
}
