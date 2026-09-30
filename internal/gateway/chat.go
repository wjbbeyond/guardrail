package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/wjbbeyond/guardrail/internal/authn"
	"github.com/wjbbeyond/guardrail/internal/cost"
	"github.com/wjbbeyond/guardrail/internal/llm"
	"github.com/wjbbeyond/guardrail/internal/provider"
	"github.com/wjbbeyond/guardrail/internal/security"
)

const maxRequestBytes = 8 << 20

func (s *Server) chatCompletions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	s.metrics.RecordRequest()
	identity := authn.FromContext(r.Context())
	if s.limits != nil {
		limit := s.limits.Allow(identity.TenantID)
		if !limit.Allowed {
			s.metrics.RecordBlocked()
			w.Header().Set("Retry-After", fmt.Sprintf("%d", limit.RetryAfter))
			writeJSON(w, http.StatusTooManyRequests, limit)
			s.recordAudit(r.Context(), auditInput{start: start, tenantID: identity.TenantID, route: r.URL.Path, status: http.StatusTooManyRequests, action: security.ActionBlock})
			return
		}
	}

	body, err := readRequestBody(r)
	if err != nil {
		s.metrics.RecordBlocked()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	chat, forwardBody, decision, err := s.normalize(body)
	if err != nil {
		s.metrics.RecordBlocked()
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	promptTokens := cost.EstimateTokens(string(forwardBody))
	if decision.Action == security.ActionBlock {
		s.metrics.RecordBlocked()
		w.Header().Set("X-GuardRail-Security", securityHeader(decision))
		writeError(w, http.StatusForbidden, "request blocked by GuardRail security policy")
		s.recordAudit(r.Context(), auditInput{start: start, tenantID: identity.TenantID, route: r.URL.Path, model: chat.Model, status: http.StatusForbidden, action: decision.Action, promptTokens: promptTokens})
		return
	}

	maxTokens := chat.MaxTokens
	reservation, budget, err := s.costs.ReserveTenant(r.Context(), identity.TenantID, chat.Model, promptTokens, maxTokens)
	if err != nil {
		s.metrics.RecordBlocked()
		s.logger.ErrorContext(r.Context(), "evaluate budget", slog.Any("err", err))
		writeError(w, http.StatusInternalServerError, "evaluate budget")
		s.recordAudit(r.Context(), auditInput{start: start, tenantID: identity.TenantID, route: r.URL.Path, model: chat.Model, status: http.StatusInternalServerError, action: security.ActionBlock, promptTokens: promptTokens})
		return
	}
	if !budget.Allowed {
		s.metrics.RecordBlocked()
		writeJSON(w, http.StatusPaymentRequired, budget)
		s.recordAudit(r.Context(), auditInput{start: start, tenantID: identity.TenantID, route: r.URL.Path, model: chat.Model, status: http.StatusPaymentRequired, action: security.ActionBlock, promptTokens: promptTokens})
		return
	}

	upstream, err := s.callProviders(r.Context(), chat, forwardBody)
	if err != nil {
		s.logger.ErrorContext(r.Context(), "all providers failed", "request_id", requestID(r.Context()))
		writeError(w, http.StatusBadGateway, "upstream request failed")
		// An ambiguous transport failure may already have incurred cost.
		usage, _ := s.settleUsage(r.Context(), reservation, chat.Model, promptTokens, maxTokens)
		s.metrics.RecordCost(usage.CostUSD)
		s.recordAudit(r.Context(), auditInput{start: start, tenantID: identity.TenantID, route: r.URL.Path, model: chat.Model, status: http.StatusBadGateway, action: decision.Action, usage: usage})
		return
	}

	w.Header().Set("X-GuardRail-Security", securityHeader(decision))
	if upstream.Streaming {
		actualPrompt, actualCompletion := s.writeStream(w, r, upstream, promptTokens, maxTokens)
		usage, _ := s.settleUsage(r.Context(), reservation, chat.Model, actualPrompt, actualCompletion)
		s.metrics.RecordCost(usage.CostUSD)
		s.recordAudit(r.Context(), auditInput{start: start, tenantID: identity.TenantID, route: r.URL.Path, provider: upstream.Provider, model: chat.Model, status: upstream.Status, action: decision.Action, usage: usage})
		return
	}

	copyHeaders(w.Header(), upstream.Header)
	w.WriteHeader(upstream.Status)
	if _, err := w.Write(upstream.Body); err != nil {
		s.logger.ErrorContext(r.Context(), "write provider response", slog.Any("err", err))
	}
	actualPrompt, completionTokens, ok := usageFromJSON(upstream.Body)
	if !ok {
		actualPrompt, completionTokens = promptTokens, maxTokens
	}
	if upstream.Status >= 400 {
		actualPrompt, completionTokens = 0, 0
	}
	usage, _ := s.settleUsage(r.Context(), reservation, chat.Model, actualPrompt, completionTokens)
	s.metrics.RecordCost(usage.CostUSD)
	s.recordAudit(r.Context(), auditInput{start: start, tenantID: identity.TenantID, route: r.URL.Path, provider: upstream.Provider, model: chat.Model, status: upstream.Status, action: decision.Action, usage: usage})
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

func usageFromJSON(body []byte) (int, int, bool) {
	var payload struct {
		Usage *struct {
			Prompt     *int `json:"prompt_tokens"`
			Completion *int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.Usage == nil || payload.Usage.Prompt == nil || payload.Usage.Completion == nil {
		return 0, 0, false
	}
	p, c := *payload.Usage.Prompt, *payload.Usage.Completion
	return p, c, p >= 0 && c >= 0
}

func (s *Server) writeStream(w http.ResponseWriter, r *http.Request, upstream *provider.UpstreamResponse, prompt, completion int) (int, int) {
	defer upstream.Stream.Close()
	copyHeaders(w.Header(), upstream.Header)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(upstream.Status)
	controller := http.NewResponseController(w)
	scanner := bufio.NewScanner(upstream.Stream)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var data []byte
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			if p, c, ok := usageFromJSON(bytes.TrimSpace(data)); ok {
				prompt, completion = p, c
			}
			data = nil
		} else if bytes.HasPrefix(line, []byte("data:")) {
			data = append(data, bytes.TrimSpace(line[5:])...)
			data = append(data, '\n')
		}
		if _, err := w.Write(append(append([]byte(nil), line...), '\n')); err != nil {
			return prompt, completion
		}
		if err := controller.Flush(); err != nil {
			return prompt, completion
		}
	}
	return prompt, completion
}

func (s *Server) settleUsage(ctx context.Context, reservation cost.Reservation, model string, prompt, completion int) (cost.Usage, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	usage, err := s.costs.Settle(ctx, reservation, model, prompt, completion)
	if err != nil {
		s.logger.ErrorContext(ctx, "settle cost usage; reservation retained", slog.Any("err", err))
	}
	return usage, err
}
