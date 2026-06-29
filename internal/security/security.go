package security

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/wjbbeyond/guardrail/internal/config"
)

type Action string

const (
	ActionAllow  Action = "allow"
	ActionWarn   Action = "warn"
	ActionBlock  Action = "block"
	ActionRedact Action = "redact"
)

type Finding struct {
	Kind  string `json:"kind"`
	Rule  string `json:"rule"`
	Match string `json:"match,omitempty"`
}

type Decision struct {
	Action   Action    `json:"action"`
	Findings []Finding `json:"findings"`
}

type Guard struct {
	promptMode string
	piiMode    string
	prompt     []*regexp.Regexp
	pii        []piiRule
}

type piiRule struct {
	name string
	re   *regexp.Regexp
}

func NewGuard(cfg config.SecurityConfig) *Guard {
	promptRules := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bignore (all )?(previous|prior|above) instructions\b`),
		regexp.MustCompile(`(?i)\bdeveloper mode\b`),
		regexp.MustCompile(`(?i)\breveal (the )?(system|developer) prompt\b`),
		regexp.MustCompile(`(?i)\bdisregard (the )?(policy|rules|instructions)\b`),
		regexp.MustCompile(`(?i)<\s*/?\s*(system|assistant|developer)\s*>`),
	}
	piiRules := []piiRule{
		{name: "email", re: regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)},
		{name: "credit_card", re: regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)},
		{name: "openai_key", re: regexp.MustCompile(`sk-[A-Za-z0-9_\-]{20,}`)},
		{name: "generic_api_key", re: regexp.MustCompile(`(?i)\b(api[_-]?key|token|secret)\s*[:=]\s*["']?[A-Za-z0-9_\-]{16,}`)},
	}
	for i, pattern := range cfg.ExtraPIIPatterns {
		compiled, err := regexp.Compile(pattern)
		if err == nil {
			piiRules = append(piiRules, piiRule{name: fmt.Sprintf("custom_%d", i+1), re: compiled})
		}
	}
	return &Guard{
		promptMode: normalizeMode(cfg.PromptInjectionMode, "warn"),
		piiMode:    normalizeMode(cfg.PIIMode, "redact"),
		prompt:     promptRules,
		pii:        piiRules,
	}
}

func (g *Guard) Inspect(prompt string) Decision {
	findings := make([]Finding, 0)
	for _, rule := range g.prompt {
		if match := rule.FindString(prompt); match != "" {
			findings = append(findings, Finding{Kind: "prompt_injection", Rule: rule.String(), Match: match})
		}
	}
	for _, rule := range g.pii {
		if match := rule.re.FindString(prompt); match != "" {
			findings = append(findings, Finding{Kind: "pii", Rule: rule.name, Match: match})
		}
	}
	return Decision{Action: g.actionFor(findings), Findings: findings}
}

func (g *Guard) Redact(text string) (string, []Finding) {
	findings := make([]Finding, 0)
	redacted := text
	for _, rule := range g.pii {
		matches := rule.re.FindAllString(redacted, -1)
		for _, match := range matches {
			findings = append(findings, Finding{Kind: "pii", Rule: rule.name, Match: match})
		}
		redacted = rule.re.ReplaceAllString(redacted, "[REDACTED_"+strings.ToUpper(rule.name)+"]")
	}
	return redacted, findings
}

func (g *Guard) actionFor(findings []Finding) Action {
	action := ActionAllow
	for _, finding := range findings {
		switch finding.Kind {
		case "prompt_injection":
			action = strongest(action, actionFromMode(g.promptMode))
		case "pii":
			action = strongest(action, actionFromMode(g.piiMode))
		}
	}
	return action
}

func actionFromMode(mode string) Action {
	switch mode {
	case "block":
		return ActionBlock
	case "redact":
		return ActionRedact
	case "warn":
		return ActionWarn
	default:
		return ActionAllow
	}
}

func strongest(left Action, right Action) Action {
	rank := map[Action]int{ActionAllow: 0, ActionWarn: 1, ActionRedact: 2, ActionBlock: 3}
	if rank[right] > rank[left] {
		return right
	}
	return left
}

func normalizeMode(mode string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "off", "warn", "redact", "block":
		return strings.ToLower(strings.TrimSpace(mode))
	default:
		return fallback
	}
}
