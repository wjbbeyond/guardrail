package gateway

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/wjbbeyond/guardrail/internal/audit"
	"github.com/wjbbeyond/guardrail/internal/cost"
	"github.com/wjbbeyond/guardrail/internal/security"
)

type auditInput struct {
	start        time.Time
	route        string
	provider     string
	model        string
	status       int
	promptTokens int
	action       security.Action
	usage        cost.Usage
}

func (s *Server) recordAudit(ctx context.Context, input auditInput) {
	status := input.status
	if status == 0 {
		status = http.StatusOK
	}
	promptTokens := input.usage.PromptTokens
	if promptTokens == 0 {
		promptTokens = input.promptTokens
	}
	event := audit.Event{
		Timestamp:        time.Now().UTC(),
		RequestID:        requestID(ctx),
		Route:            input.route,
		Provider:         input.provider,
		Model:            input.model,
		Status:           status,
		PromptTokens:     promptTokens,
		CompletionTokens: input.usage.CompletionTokens,
		CostUSD:          input.usage.CostUSD,
		SecurityAction:   string(input.action),
		LatencyMillis:    time.Since(input.start).Milliseconds(),
	}
	if err := s.audit.Record(ctx, event); err != nil {
		s.logger.ErrorContext(ctx, "record audit event", slog.Any("err", err))
	}
}
