package orchestrator

import (
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestEmailDomain(t *testing.T) {
	cases := map[string]string{
		"alice@contractor.com": "contractor.com",
		"BOB@Corp.COM":         "corp.com",
		"noatsign":             "",
		"trailing@":            "",
		"":                     "",
	}
	for in, want := range cases {
		if got := emailDomain(in); got != want {
			t.Errorf("emailDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchAny(t *testing.T) {
	if !matchAny(nil, "openai") {
		t.Error("empty patterns should be a wildcard (true)")
	}
	if !matchAny([]string{"openai"}, "OpenAI") {
		t.Error("should match case-insensitively")
	}
	if matchAny([]string{"openai"}, "anthropic") {
		t.Error("should not match a different value")
	}
	if !matchAny([]string{"name", "openai"}, "x", "openai") {
		t.Error("should match any candidate against any pattern")
	}
}

func TestPolicyRuleMatchesAndDeny(t *testing.T) {
	rc := aiReqContext{
		ServiceSlug: "claude-api-access", ServiceCategory: "api_access",
		ProviderName: "openai-main", ProviderType: "openai", Owner: "ext@contractor.com",
	}
	deny := aiPolicyRule{Effect: "deny", Providers: []string{"openai"}, OwnerDomains: []string{"contractor.com"}}
	if !deny.isDeny() || !deny.matches(rc) {
		t.Error("deny rule should match the contractor+openai request")
	}
	if (aiPolicyRule{Effect: "deny", Providers: []string{"anthropic"}}).matches(rc) {
		t.Error("rule for a different provider should not match")
	}
	if (aiPolicyRule{Effect: "allow"}).isDeny() {
		t.Error("effect=allow should not be a deny")
	}
	if !(aiPolicyRule{}).isDeny() {
		t.Error("empty effect should default to deny")
	}
}

func TestBudgetAppliesToRequest(t *testing.T) {
	tid := uuid.New()
	rc := aiReqContext{
		ProviderName: "openai-main", Owner: "a@corp.com", Workspace: "team-a",
		Tenant: pgtype.UUID{Bytes: tid, Valid: true},
	}
	checks := []struct {
		name string
		b    db.AiBudget
		want bool
	}{
		{"global always applies", db.AiBudget{Scope: "global"}, true},
		{"empty scope == global", db.AiBudget{Scope: ""}, true},
		{"provider match", db.AiBudget{Scope: "provider", ScopeRef: "openai-main"}, true},
		{"provider mismatch", db.AiBudget{Scope: "provider", ScopeRef: "anthropic-main"}, false},
		{"owner match", db.AiBudget{Scope: "owner", ScopeRef: "a@corp.com"}, true},
		{"workspace match", db.AiBudget{Scope: "workspace", ScopeRef: "team-a"}, true},
		{"tenant match", db.AiBudget{Scope: "tenant", ScopeRef: tid.String()}, true},
		{"tenant mismatch", db.AiBudget{Scope: "tenant", ScopeRef: uuid.New().String()}, false},
	}
	for _, c := range checks {
		if got := budgetAppliesToRequest(c.b, rc); got != c.want {
			t.Errorf("%s: budgetAppliesToRequest = %v, want %v", c.name, got, c.want)
		}
	}
}
