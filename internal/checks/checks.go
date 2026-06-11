// Package checks is OPORD's compliance / guardrails engine - a Soundcheck-style
// governance layer over the provisioned-resource inventory (ADR-0014).
//
// A Check is a single rule (e.g. "every account has a budget", "no public
// buckets", "k8s version is supported"). The Engine runs the registered checks
// over a set of Subjects (a normalized view of each provisioned resource) and
// produces Results, which aggregate into a Scorecard (overall score + per
// category / account / environment breakdown + the actionable list of failures).
//
// The MVP evaluates entirely from each resource's stored spec + observed state
// (no extra cloud calls) - fast, free, and tenant-scoped like the cost report.
// Live-state checks (querying the provider) are a later phase.
package checks

import "sort"

// Severity ranks a check's importance.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// severityRank orders severities for sorting (higher = more severe).
func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	}
	return 0
}

// Status is the outcome of evaluating one check against one subject.
type Status string

const (
	StatusPass Status = "pass" // the resource satisfies the rule
	StatusFail Status = "fail" // the resource violates the rule
	StatusSkip Status = "skip" // the rule does not apply to this resource
)

// Categories group checks by concern.
const (
	CategorySecurity    = "security"
	CategoryCost        = "cost"
	CategoryReliability = "reliability"
	CategoryTagging     = "tagging"
	CategoryGovernance  = "governance"
)

// Subject is a normalized, provider-neutral view of one provisioned resource.
// The orchestrator builds these from the tenant-scoped resource lists.
type Subject struct {
	Name         string
	Kind         string // vm, cluster, database, table, object-storage, secret, queue, cache, function, stack, account
	Provider     string // provider NAME (e.g. OPORD-AWS)
	ProviderType string // aws | azure | gcp | vsphere | proxmox (optional; "" if unknown)
	Environment  string // prod | stage | dev
	Status       string // ready | failed | provisioning | destroying | destroyed | ...
	Account      string // landing-zone target (target_account) or "" for the provider default
	Tenant       string // owning tenant id (for MSP per-client grouping)
	Spec         map[string]any
	Observed     map[string]any
}

// Check is one compliance rule.
type Check struct {
	ID          string
	Title       string
	Description string
	Category    string
	Severity    Severity
	Kinds       []string // resource kinds this applies to; empty = all kinds
	Remediation string   // human-readable fix
	// Eval returns the outcome and a short message. Return StatusSkip when the
	// rule is not meaningful for this subject (skips never count against score).
	Eval func(Subject) (Status, string)
}

func (c Check) applies(kind string) bool {
	if len(c.Kinds) == 0 {
		return true
	}
	for _, k := range c.Kinds {
		if k == kind {
			return true
		}
	}
	return false
}

// Result is the evaluation of one Check against one Subject.
type Result struct {
	CheckID     string   `json:"checkId"`
	Title       string   `json:"title"`
	Category    string   `json:"category"`
	Severity    Severity `json:"severity"`
	Remediation string   `json:"remediation,omitempty"`
	Subject     string   `json:"subject"`
	Kind        string   `json:"kind"`
	Provider    string   `json:"provider"`
	Account     string   `json:"account,omitempty"`
	Environment string   `json:"environment,omitempty"`
	Status      Status   `json:"status"`
	Message     string   `json:"message"`
}

// Engine runs a fixed set of checks over subjects.
type Engine struct{ checks []Check }

// NewEngine builds an engine. With no checks it uses BuiltinChecks().
func NewEngine(checks ...Check) *Engine {
	if len(checks) == 0 {
		checks = BuiltinChecks()
	}
	return &Engine{checks: checks}
}

// Checks returns the registered checks (for listing the catalog).
func (e *Engine) Checks() []Check { return e.checks }

// Run evaluates every applicable check against every subject. Subjects already
// in a terminal "destroyed" state are skipped entirely (nothing to govern).
func (e *Engine) Run(subjects []Subject) []Result {
	out := make([]Result, 0, len(subjects)*2)
	for _, s := range subjects {
		if s.Status == "destroyed" {
			continue
		}
		for _, c := range e.checks {
			if !c.applies(s.Kind) {
				continue
			}
			st, msg := c.Eval(s)
			out = append(out, Result{
				CheckID:     c.ID,
				Title:       c.Title,
				Category:    c.Category,
				Severity:    c.Severity,
				Remediation: c.Remediation,
				Subject:     s.Name,
				Kind:        s.Kind,
				Provider:    s.Provider,
				Account:     s.Account,
				Environment: s.Environment,
				Status:      st,
				Message:     msg,
			})
		}
	}
	return out
}

// CategoryScore / GroupScore / CheckSummary are scorecard breakdowns.
type CategoryScore struct {
	Category string  `json:"category"`
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Score    float64 `json:"score"`
}

type GroupScore struct {
	Name   string  `json:"name"`
	Passed int     `json:"passed"`
	Failed int     `json:"failed"`
	Score  float64 `json:"score"`
}

type CheckSummary struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Category string   `json:"category"`
	Severity Severity `json:"severity"`
	Passed   int      `json:"passed"`
	Failed   int      `json:"failed"`
}

// Scorecard aggregates results into an overall posture.
type Scorecard struct {
	Score           float64         `json:"score"` // % passing of non-skip evaluations (0-100)
	Evaluated       int             `json:"evaluated"`
	Passed          int             `json:"passed"`
	Failed          int             `json:"failed"`
	CriticalFailing int             `json:"criticalFailing"`
	WarningFailing  int             `json:"warningFailing"`
	ByCategory      []CategoryScore `json:"byCategory"`
	ByAccount       []GroupScore    `json:"byAccount"`
	ByEnvironment   []GroupScore    `json:"byEnvironment"`
	Checks          []CheckSummary  `json:"checks"`
	Failures        []Result        `json:"failures"` // failing results only, most-severe first
}

func pct(passed, failed int) float64 {
	total := passed + failed
	if total == 0 {
		return 100
	}
	return float64(passed) / float64(total) * 100
}

// Score aggregates raw results into a Scorecard. Skips are excluded from all
// counts and from the score (a rule that does not apply neither helps nor hurts).
func Score(results []Result) Scorecard {
	sc := Scorecard{}
	cat := map[string]*CategoryScore{}
	acct := map[string]*GroupScore{}
	env := map[string]*GroupScore{}
	chk := map[string]*CheckSummary{}
	catOrder := []string{}
	acctOrder := []string{}
	envOrder := []string{}
	chkOrder := []string{}

	for _, r := range results {
		if r.Status == StatusSkip {
			continue
		}
		passed := r.Status == StatusPass
		if passed {
			sc.Passed++
		} else {
			sc.Failed++
			if r.Severity == SeverityCritical {
				sc.CriticalFailing++
			} else if r.Severity == SeverityWarning {
				sc.WarningFailing++
			}
			sc.Failures = append(sc.Failures, r)
		}

		if cat[r.Category] == nil {
			cat[r.Category] = &CategoryScore{Category: r.Category}
			catOrder = append(catOrder, r.Category)
		}
		acctKey := r.Account
		if acctKey == "" {
			acctKey = "(provider default)"
		}
		if acct[acctKey] == nil {
			acct[acctKey] = &GroupScore{Name: acctKey}
			acctOrder = append(acctOrder, acctKey)
		}
		envKey := r.Environment
		if envKey == "" {
			envKey = "(none)"
		}
		if env[envKey] == nil {
			env[envKey] = &GroupScore{Name: envKey}
			envOrder = append(envOrder, envKey)
		}
		if chk[r.CheckID] == nil {
			chk[r.CheckID] = &CheckSummary{ID: r.CheckID, Title: r.Title, Category: r.Category, Severity: r.Severity}
			chkOrder = append(chkOrder, r.CheckID)
		}
		if passed {
			cat[r.Category].Passed++
			acct[acctKey].Passed++
			env[envKey].Passed++
			chk[r.CheckID].Passed++
		} else {
			cat[r.Category].Failed++
			acct[acctKey].Failed++
			env[envKey].Failed++
			chk[r.CheckID].Failed++
		}
	}

	sc.Evaluated = sc.Passed + sc.Failed
	sc.Score = pct(sc.Passed, sc.Failed)
	for _, k := range catOrder {
		c := cat[k]
		c.Score = pct(c.Passed, c.Failed)
		sc.ByCategory = append(sc.ByCategory, *c)
	}
	for _, k := range acctOrder {
		g := acct[k]
		g.Score = pct(g.Passed, g.Failed)
		sc.ByAccount = append(sc.ByAccount, *g)
	}
	for _, k := range envOrder {
		g := env[k]
		g.Score = pct(g.Passed, g.Failed)
		sc.ByEnvironment = append(sc.ByEnvironment, *g)
	}
	for _, k := range chkOrder {
		sc.Checks = append(sc.Checks, *chk[k])
	}

	// Most-severe failures first, then by check id for stable ordering.
	sort.SliceStable(sc.Failures, func(i, j int) bool {
		ri, rj := severityRank(sc.Failures[i].Severity), severityRank(sc.Failures[j].Severity)
		if ri != rj {
			return ri > rj
		}
		return sc.Failures[i].CheckID < sc.Failures[j].CheckID
	})
	// Worst-scoring categories first.
	sort.SliceStable(sc.ByCategory, func(i, j int) bool { return sc.ByCategory[i].Score < sc.ByCategory[j].Score })
	sort.SliceStable(sc.ByAccount, func(i, j int) bool { return sc.ByAccount[i].Score < sc.ByAccount[j].Score })
	return sc
}
