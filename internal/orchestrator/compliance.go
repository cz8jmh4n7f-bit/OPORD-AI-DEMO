package orchestrator

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/checks"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
)

// subjectFromResource normalizes a generic resource row (kind=vm/database/...)
// into a compliance Subject: parse the stored spec + observed JSON, lift the
// deploy-into target and tags so the checks can read them.
func subjectFromResource(r db.Resource, kind, provider string) checks.Subject {
	spec := map[string]any{}
	obs := map[string]any{}
	_ = json.Unmarshal(r.Spec, &spec)
	if len(r.Observed) > 0 {
		_ = json.Unmarshal(r.Observed, &obs)
	}
	acct, _ := spec["target_account"].(string)
	return checks.Subject{
		Name:        r.Name,
		Kind:        kind,
		Provider:    provider,
		Environment: r.Environment,
		Status:      r.Status,
		Account:     acct,
		Spec:        spec,
		Observed:    obs,
	}
}

// ComplianceReport runs the guardrail engine (internal/checks) over every
// resource visible to the caller (tenant-scoped via the same List* methods the
// cost report uses) and returns the aggregated scorecard. No cloud calls - the
// MVP evaluates from each resource's stored spec + observed state.
func (s *Service) ComplianceReport(ctx context.Context) (*checks.Scorecard, error) {
	var subjects []checks.Subject

	vms, err := s.ListVMs(ctx)
	if err != nil {
		return nil, err
	}
	for _, v := range vms {
		subjects = append(subjects, subjectFromResource(v.Resource, "vm", v.Provider))
	}

	clusters, err := s.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range clusters {
		spec := map[string]any{}
		obs := map[string]any{}
		_ = json.Unmarshal(c.Cluster.DesiredSpec, &spec)
		if len(c.Cluster.ObservedState) > 0 {
			_ = json.Unmarshal(c.Cluster.ObservedState, &obs)
		}
		acct, _ := spec["target_account"].(string)
		subjects = append(subjects, checks.Subject{
			Name:         c.Cluster.Name,
			Kind:         "cluster",
			Provider:     c.Provider,
			ProviderType: c.ProviderType,
			Environment:  c.Cluster.Environment,
			Status:       c.Cluster.Status,
			Account:      acct,
			Spec:         spec,
			Observed:     obs,
		})
	}

	dbs, err := s.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range dbs {
		subjects = append(subjects, subjectFromResource(d.Resource, "database", d.Provider))
	}

	tables, err := s.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		subjects = append(subjects, subjectFromResource(t.Resource, "table", t.Provider))
	}

	buckets, err := s.ListS3(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range buckets {
		subjects = append(subjects, subjectFromResource(b.Resource, "object-storage", b.Provider))
	}

	secrets, err := s.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}
	for _, sec := range secrets {
		subjects = append(subjects, subjectFromResource(sec.Resource, "secret", sec.Provider))
	}

	queues, err := s.ListQueues(ctx)
	if err != nil {
		return nil, err
	}
	for _, q := range queues {
		subjects = append(subjects, subjectFromResource(q.Resource, "queue", q.Provider))
	}

	caches, err := s.ListCaches(ctx)
	if err != nil {
		return nil, err
	}
	for _, ca := range caches {
		subjects = append(subjects, subjectFromResource(ca.Resource, "cache", ca.Provider))
	}

	functions, err := s.ListFunctions(ctx)
	if err != nil {
		return nil, err
	}
	for _, fn := range functions {
		subjects = append(subjects, subjectFromResource(fn.Resource, "function", fn.Provider))
	}

	stacks, err := s.ListStacks(ctx)
	if err != nil {
		return nil, err
	}
	for _, st := range stacks {
		subjects = append(subjects, subjectFromResource(st.Resource, "stack", st.Provider))
	}

	accounts, err := s.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	for _, a := range accounts {
		subjects = append(subjects, subjectFromResource(a.Resource, "account", a.Provider))
	}

	// AI access grants - governed entitlements (owner/expiry/access-review).
	aiInstances, err := s.ListAIInstances(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for _, inst := range aiInstances {
		if inst.Status == "revoked" || inst.Status == "expired" {
			continue
		}
		ws := ""
		if inst.Workspace != "" {
			ws = inst.Workspace
		}
		overdue := (inst.Status == "active" || inst.Status == "suspended") &&
			inst.ExpiresAt.Valid && inst.ExpiresAt.Time.Before(now)
		subjects = append(subjects, checks.Subject{
			Name:        inst.ProviderName + "/" + inst.Owner,
			Kind:        "ai-access",
			Provider:    inst.ProviderName,
			Environment: ws,
			Status:      inst.Status,
			Observed: map[string]any{
				"owner":      inst.Owner,
				"workspace":  ws,
				"has_expiry": inst.ExpiresAt.Valid,
				"overdue":    overdue,
			},
		})
	}

	results := checks.NewEngine().Run(subjects)
	sc := checks.Score(results)
	return &sc, nil
}
