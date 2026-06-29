package metrics

import (
	"fmt"
	"strings"
	"sync/atomic"
)

type Registry struct {
	requests  atomic.Int64
	blocked   atomic.Int64
	failovers atomic.Int64
	costMicro atomic.Int64
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) RecordRequest() {
	r.requests.Add(1)
}

func (r *Registry) RecordBlocked() {
	r.blocked.Add(1)
}

func (r *Registry) RecordFailover() {
	r.failovers.Add(1)
}

func (r *Registry) RecordCost(costUSD float64) {
	r.costMicro.Add(int64(costUSD * 1_000_000))
}

func (r *Registry) Prometheus() string {
	var b strings.Builder
	writeMetric(&b, "guardrail_requests_total", "Total chat completion requests.", float64(r.requests.Load()))
	writeMetric(&b, "guardrail_blocked_requests_total", "Requests blocked by security or budget guardrails.", float64(r.blocked.Load()))
	writeMetric(&b, "guardrail_provider_failovers_total", "Provider failover attempts.", float64(r.failovers.Load()))
	writeMetric(&b, "guardrail_estimated_cost_usd_total", "Estimated total LLM spend in USD.", float64(r.costMicro.Load())/1_000_000)
	return b.String()
}

func writeMetric(b *strings.Builder, name string, help string, value float64) {
	fmt.Fprintf(b, "# HELP %s %s\n", name, help)
	fmt.Fprintf(b, "# TYPE %s counter\n", name)
	fmt.Fprintf(b, "%s %.6f\n", name, value)
}
