package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// aiReqContext carries the facts an AI request is evaluated against. It is built
// at request creation and again at approval so governance is checked on both.
type aiReqContext struct {
	ServiceID       uuid.UUID
	ServiceSlug     string
	ServiceCategory string
	ProviderName    string
	ProviderType    string
	ProviderID      string
	Owner           string
	Workspace       string
	Tenant          pgtype.UUID
}

// aiEnforcementError is returned when an active governance rule BLOCKS a request
// (as opposed to a system error). The API maps it to HTTP 403 so the requester
// sees the reason instead of a generic failure.
type aiEnforcementError struct{ reasons []string }

func (e *aiEnforcementError) Error() string {
	return "blocked by AI governance: " + strings.Join(e.reasons, "; ")
}

// IsAIEnforcementError reports whether err is a governance block.
func IsAIEnforcementError(err error) bool {
	var e *aiEnforcementError
	return errors.As(err, &e)
}

// aiPolicyRule is the (minimal, documented) shape stored in
// ai_access_policies.rules. A rule SELECTS requests: every non-empty selector
// must match (AND); an empty selector is a wildcard. An active rule whose effect
// is "deny" (the default) BLOCKS any request it selects; an active "allow" rule
// that matches WHITELISTS the request past all deny rules (allow overrides deny).
//
// Example - deny external contractors any OpenAI access:
//
//	{"effect":"deny","providers":["openai"],"owner_domains":["contractor.com"]}
type aiPolicyRule struct {
	Effect       string   `json:"effect"`
	Providers    []string `json:"providers"`     // provider name or type
	Categories   []string `json:"categories"`    // service category
	Services     []string `json:"services"`      // service slug
	OwnerDomains []string `json:"owner_domains"` // owner email domain
}

func (r aiPolicyRule) isDeny() bool {
	return !strings.EqualFold(strings.TrimSpace(r.Effect), "allow")
}

func (r aiPolicyRule) matches(rc aiReqContext) bool {
	return matchAny(r.Providers, rc.ProviderName, rc.ProviderType) &&
		matchAny(r.Categories, rc.ServiceCategory) &&
		matchAny(r.Services, rc.ServiceSlug) &&
		matchAny(r.OwnerDomains, emailDomain(rc.Owner))
}

// matchAny reports whether patterns is empty (wildcard) or any pattern equals
// (case-insensitively) any candidate.
func matchAny(patterns []string, candidates ...string) bool {
	if len(patterns) == 0 {
		return true
	}
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			return true
		}
		for _, c := range candidates {
			if strings.EqualFold(p, strings.TrimSpace(c)) {
				return true
			}
		}
	}
	return false
}

// emailDomain returns the lowercased domain of an owner email ("" if none).
func emailDomain(owner string) string {
	at := strings.LastIndex(owner, "@")
	if at < 0 || at == len(owner)-1 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(owner[at+1:]))
}

// budgetAppliesToRequest reports whether a budget's scope covers this request.
func budgetAppliesToRequest(b db.AiBudget, rc aiReqContext) bool {
	ref := strings.TrimSpace(b.ScopeRef)
	switch strings.ToLower(strings.TrimSpace(b.Scope)) {
	case "", "global":
		return true
	case "provider":
		return ref == "" || strings.EqualFold(rc.ProviderName, ref) || (rc.ProviderID != "" && rc.ProviderID == ref)
	case "owner":
		return strings.EqualFold(rc.Owner, ref)
	case "workspace":
		return ref == "" || strings.EqualFold(rc.Workspace, ref)
	case "tenant":
		if ref == "" {
			return !rc.Tenant.Valid
		}
		return rc.Tenant.Valid && uuid.UUID(rc.Tenant.Bytes).String() == ref
	default:
		return false
	}
}

func scopeLabel(b db.AiBudget) string {
	scope := strings.TrimSpace(b.Scope)
	if scope == "" {
		scope = "global"
	}
	if ref := strings.TrimSpace(b.ScopeRef); ref != "" {
		return scope + ":" + ref
	}
	return scope
}

// evaluateAIGovernance enforces active policies, quotas, and budgets against a
// pending AI request. It BLOCKS (returns *aiEnforcementError) on a hard
// violation: an active deny policy, a metric "instances"/"seats" quota with
// enforcement "block" at/over its limit, or an applicable budget at its hard
// threshold. Soft signals (warn-quotas, budgets in the warning band) are audited
// but allowed. All inputs come from the existing tenant-scoped List* methods, so
// no new queries/migrations are needed.
func (s *Service) evaluateAIGovernance(ctx context.Context, rc aiReqContext) error {
	var hard, soft []string

	// 1) Policy guardrails: a matching active ALLOW rule whitelists the request
	//    past DENY rules; otherwise every matching active DENY blocks it.
	if policies, err := s.ListAIAccessPolicies(ctx); err == nil {
		allowed := false
		var denied []string
		for _, p := range policies {
			if p.Status != "active" {
				continue
			}
			var rule aiPolicyRule
			if json.Unmarshal(p.Rules, &rule) != nil || !rule.matches(rc) {
				continue
			}
			if rule.isDeny() {
				denied = append(denied, fmt.Sprintf("policy %q denies this request", p.Name))
			} else {
				allowed = true
			}
		}
		if !allowed {
			hard = append(hard, denied...)
		}
	}

	// 2) Seat/instance quotas. Token & cost quotas are enforced on the gateway path.
	if quotas, err := s.ListAIQuotas(ctx); err == nil {
		var instances []db.ListAIServiceInstancesRow
		loaded := false
		for _, q := range quotas {
			metric := strings.ToLower(strings.TrimSpace(q.Metric))
			if metric != "instances" && metric != "seats" {
				continue
			}
			scoped := q.ServiceID.Valid
			if scoped && uuid.UUID(q.ServiceID.Bytes) != rc.ServiceID {
				continue
			}
			if !loaded {
				instances, _ = s.ListAIInstances(ctx)
				loaded = true
			}
			active := 0
			for _, in := range instances {
				if in.Status != "active" {
					continue
				}
				if scoped && in.ServiceID != rc.ServiceID {
					continue
				}
				active++
			}
			if float64(active) >= q.LimitQuantity {
				msg := fmt.Sprintf("quota reached (%d/%.0f active %s)", active, q.LimitQuantity, metric)
				if q.Enforcement == "block" {
					hard = append(hard, msg)
				} else {
					soft = append(soft, msg)
				}
			}
		}
	}

	// 3) Budget spend gate (reuses the computed ok/warning/hard_limit status).
	if budgets, err := s.ListAIBudgetSummaries(ctx); err == nil {
		for _, b := range budgets {
			if !budgetAppliesToRequest(b.Budget, rc) {
				continue
			}
			switch b.Status {
			case "hard_limit":
				hard = append(hard, fmt.Sprintf("budget exhausted ($%.2f/$%.2f %s)", b.ActualUSD, b.Budget.LimitUsd, scopeLabel(b.Budget)))
			case "warning":
				soft = append(soft, fmt.Sprintf("budget at %.0f%% (%s)", b.UsagePct, scopeLabel(b.Budget)))
			}
		}
	}

	for _, w := range soft {
		s.emitAIAudit(ctx, "ai_request", uuid.Nil, "governance_warning", w, map[string]any{"service": rc.ServiceSlug, "owner": rc.Owner}, rc.Owner)
	}
	if len(hard) > 0 {
		s.emitAIAudit(ctx, "ai_request", uuid.Nil, "governance_blocked", strings.Join(hard, "; "), map[string]any{"service": rc.ServiceSlug, "owner": rc.Owner}, rc.Owner)
		return &aiEnforcementError{reasons: hard}
	}
	return nil
}

// evaluateGatewayBudget blocks an AI gateway proxy call when a global or
// provider budget is at its hard limit (the spend gate for the proxy path). It
// fails OPEN on a read error so an infra blip never wedges the proxy.
func (s *Service) evaluateGatewayBudget(ctx context.Context, providerName string) error {
	budgets, err := s.ListAIBudgetSummaries(ctx)
	if err != nil {
		return nil
	}
	rc := aiReqContext{ProviderName: providerName}
	for _, b := range budgets {
		scope := strings.ToLower(strings.TrimSpace(b.Budget.Scope))
		if scope != "" && scope != "global" && scope != "provider" {
			continue
		}
		if b.Status == "hard_limit" && budgetAppliesToRequest(b.Budget, rc) {
			s.emitAIAudit(ctx, "ai_gateway", uuid.Nil, "governance_blocked", "budget exhausted", map[string]any{"provider": providerName}, "")
			return &aiEnforcementError{reasons: []string{fmt.Sprintf("budget exhausted for provider %q", providerName)}}
		}
	}
	// Token / cost / request quotas are enforced HERE - the consumption point.
	// Global (unscoped) block-quotas only; a service-scoped token quota can't be
	// mapped to a raw proxy call.
	if quotas, err := s.ListAIQuotas(ctx); err == nil {
		usage, _ := s.ListAIUsageRecords(ctx)
		for _, q := range quotas {
			if q.ServiceID.Valid || q.Enforcement != "block" {
				continue
			}
			metric := strings.ToLower(strings.TrimSpace(q.Metric))
			if metric != "tokens" && metric != "cost_usd" && metric != "requests" {
				continue
			}
			start := aiBudgetPeriodStart(q.Period)
			var total float64
			for _, u := range usage {
				if !strings.EqualFold(u.ProviderName, providerName) || u.PeriodStart.Before(start) {
					continue
				}
				switch metric {
				case "tokens":
					if strings.EqualFold(u.Metric, "tokens") {
						total += u.Quantity
					}
				case "cost_usd":
					total += u.CostUsd
				case "requests":
					total++
				}
			}
			if total >= q.LimitQuantity {
				s.emitAIAudit(ctx, "ai_gateway", uuid.Nil, "governance_blocked", metric+" quota reached", map[string]any{"provider": providerName}, "")
				return &aiEnforcementError{reasons: []string{fmt.Sprintf("%s quota reached (%.0f/%.0f) for provider %q this %s", metric, total, q.LimitQuantity, providerName, q.Period)}}
			}
		}
	}
	return nil
}
