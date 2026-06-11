package checks

import "testing"

func TestMinorVersion(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"1.33", 133, true},
		{"1.33.2-gke", 133, true},
		{"1.31", 131, true},
		{"1.30", 130, true},
		{"", 0, false},
		{"latest", 0, false},
		{"1", 0, false},
	}
	for _, c := range cases {
		got, ok := minorVersion(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("minorVersion(%q)=%d,%v want %d,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestTagValue(t *testing.T) {
	spec := map[string]any{
		"owner": "alice",
		"tags":  map[string]any{"cost_center": "eng"},
	}
	if got := tagValue(spec, "owner"); got != "alice" {
		t.Errorf("owner=%q want alice", got)
	}
	if got := tagValue(spec, "cost_center"); got != "eng" {
		t.Errorf("cost_center=%q want eng (nested)", got)
	}
	if got := tagValue(spec, "missing"); got != "" {
		t.Errorf("missing=%q want empty", got)
	}
}

// eval finds a builtin check by id and evaluates it against a subject.
func eval(t *testing.T, id string, s Subject) (Status, string) {
	t.Helper()
	for _, c := range BuiltinChecks() {
		if c.ID == id {
			return c.Eval(s)
		}
	}
	t.Fatalf("check %q not found", id)
	return "", ""
}

func TestBuiltinChecks(t *testing.T) {
	cases := []struct {
		name  string
		check string
		subj  Subject
		want  Status
	}{
		{"owner present", "owner-tag", Subject{Spec: map[string]any{"owner": "bob"}}, StatusPass},
		{"owner missing", "owner-tag", Subject{Spec: map[string]any{}}, StatusFail},
		{"ttl set", "ttl-set", Subject{Kind: "vm", Spec: map[string]any{"ttl_hours": float64(24)}}, StatusPass},
		{"ttl missing", "ttl-set", Subject{Kind: "vm", Spec: map[string]any{}}, StatusFail},
		{"budget set", "account-budget", Subject{Kind: "account", Spec: map[string]any{"monthly_budget_usd": float64(500)}}, StatusPass},
		{"budget missing", "account-budget", Subject{Kind: "account", Spec: map[string]any{}}, StatusFail},
		{"bucket private (default)", "storage-not-public", Subject{Kind: "object-storage", Spec: map[string]any{}}, StatusPass},
		{"bucket public flag", "storage-not-public", Subject{Kind: "object-storage", Spec: map[string]any{"public": true}}, StatusFail},
		{"bucket bpa false", "storage-not-public", Subject{Kind: "object-storage", Spec: map[string]any{"block_public_access": false}}, StatusFail},
		{"db clean", "db-no-plaintext-password", Subject{Kind: "database", Observed: map[string]any{"password_secret": "opord/databases/x"}}, StatusPass},
		{"db leaked pw", "db-no-plaintext-password", Subject{Kind: "database", Observed: map[string]any{"admin_password": "hunter2"}}, StatusFail},
		{"default vpcs removed", "account-default-vpcs-removed", Subject{Kind: "account", Spec: map[string]any{}}, StatusPass},
		{"default vpcs skipped", "account-default-vpcs-removed", Subject{Kind: "account", Spec: map[string]any{"skip": map[string]any{"delete_default_vpcs": true}}}, StatusFail},
		{"azure account skipped", "account-default-vpcs-removed", Subject{Kind: "account", Spec: map[string]any{"azure_mode": "adopt"}}, StatusSkip},
		{"not failed", "not-failed", Subject{Status: "ready"}, StatusPass},
		{"failed", "not-failed", Subject{Status: "failed"}, StatusFail},
		{"k8s supported", "cluster-k8s-supported", Subject{Kind: "cluster", Spec: map[string]any{"kubernetes_version": "1.33"}}, StatusPass},
		{"k8s old", "cluster-k8s-supported", Subject{Kind: "cluster", Spec: map[string]any{"kubernetes_version": "1.28"}}, StatusFail},
		{"k8s unpinned", "cluster-k8s-supported", Subject{Kind: "cluster", Spec: map[string]any{}}, StatusSkip},
	}
	for _, c := range cases {
		got, _ := eval(t, c.check, c.subj)
		if got != c.want {
			t.Errorf("%s: %s = %v want %v", c.name, c.check, got, c.want)
		}
	}
}

func TestEngineAndScore(t *testing.T) {
	subjects := []Subject{
		{Name: "web", Kind: "vm", Provider: "aws", Environment: "prod", Status: "ready", Spec: map[string]any{"owner": "alice", "ttl_hours": float64(48)}},
		{Name: "leaky", Kind: "object-storage", Provider: "aws", Environment: "prod", Status: "ready", Spec: map[string]any{"public": true}},
		{Name: "gone", Kind: "vm", Provider: "aws", Status: "destroyed", Spec: map[string]any{}}, // skipped entirely
	}
	results := NewEngine().Run(subjects)
	if len(results) == 0 {
		t.Fatal("no results")
	}
	for _, r := range results {
		if r.Subject == "gone" {
			t.Errorf("destroyed subject should be skipped, got result %s", r.CheckID)
		}
	}
	sc := Score(results)
	if sc.Evaluated == 0 {
		t.Fatal("nothing evaluated")
	}
	if sc.CriticalFailing < 1 {
		t.Errorf("expected the public bucket to be a critical failure, got %d", sc.CriticalFailing)
	}
	// The public-bucket critical failure must surface first in Failures.
	if len(sc.Failures) == 0 || sc.Failures[0].Severity != SeverityCritical {
		t.Errorf("most-severe failure should sort first")
	}
	if sc.Score < 0 || sc.Score > 100 {
		t.Errorf("score out of range: %v", sc.Score)
	}
}
