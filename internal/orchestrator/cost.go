package orchestrator

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/pricing"
)

// CostLine is one resource's estimated monthly cost.
type CostLine struct {
	Name        string
	Kind        string
	Provider    string
	Environment string
	Status      string
	MonthlyUSD  float64
	Owner       string
	Project     string
	CostCenter  string
	TTLHours    int
	RiskFlags   []string
}

// CostReport is the estimated monthly spend across all active resources.
type CostReport struct {
	Lines    []CostLine
	TotalUSD float64
}

func (r *CostReport) add(name, kind, provider, env, status string, monthly float64, spec json.RawMessage) {
	meta := allocationFromSpec(spec)
	r.Lines = append(r.Lines, CostLine{
		Name:        name,
		Kind:        kind,
		Provider:    provider,
		Environment: env,
		Status:      status,
		MonthlyUSD:  monthly,
		Owner:       meta.owner,
		Project:     meta.project,
		CostCenter:  meta.costCenter,
		TTLHours:    meta.ttlHours,
		RiskFlags:   meta.riskFlags,
	})
	r.TotalUSD += monthly
}

type allocationMeta struct {
	owner      string
	project    string
	costCenter string
	ttlHours   int
	riskFlags  []string
}

func allocationFromSpec(raw json.RawMessage) allocationMeta {
	var m map[string]any
	_ = json.Unmarshal(raw, &m)
	meta := allocationMeta{
		owner:      readSpecString(m, "owner"),
		project:    readSpecString(m, "project"),
		costCenter: readSpecString(m, "cost_center"),
		ttlHours:   readSpecInt(m, "ttl_hours"),
	}
	if tags, ok := m["tags"].(map[string]any); ok {
		if meta.owner == "" {
			meta.owner = readSpecString(tags, "owner")
		}
		if meta.project == "" {
			meta.project = readSpecString(tags, "project")
		}
		if meta.costCenter == "" {
			meta.costCenter = readSpecString(tags, "cost_center")
		}
	}
	if readSpecBool(m, "public_ip") {
		meta.riskFlags = append(meta.riskFlags, "public-ip")
	}
	if readSpecBool(m, "public_access") {
		meta.riskFlags = append(meta.riskFlags, "public-db")
	}
	if v, ok := m["block_public_access"]; ok && !truthy(v) {
		meta.riskFlags = append(meta.riskFlags, "public-object-storage")
	}
	if strings.EqualFold(readSpecString(m, "billing_mode"), "PROVISIONED") {
		meta.riskFlags = append(meta.riskFlags, "fixed-capacity")
	}
	return meta
}

func readSpecString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func readSpecInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return 0
}

func readSpecBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	return truthy(m[key])
}

func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return strings.EqualFold(x, "true") || x == "1" || strings.EqualFold(x, "yes")
	default:
		return false
	}
}

// CostReport estimates monthly spend for every non-destroyed OPORD-managed
// resource with a provider-neutral pricing model. Exact billed cost comes from
// FOCUS ingestion; this report powers guardrails before the bill arrives.
func (s *Service) CostReport(ctx context.Context) (*CostReport, error) {
	rep := &CostReport{}

	vms, err := s.ListVMs(ctx)
	if err != nil {
		return nil, err
	}
	for _, v := range vms {
		if v.Resource.Status == "destroyed" {
			continue
		}
		rep.add(v.Resource.Name, "vm", v.Provider, v.Resource.Environment, v.Resource.Status, pricing.VM(v.Spec), v.Resource.Spec)
	}

	clusters, err := s.ListClusters(ctx)
	if err != nil {
		return nil, err
	}
	for _, c := range clusters {
		if c.Cluster.Status == "destroyed" {
			continue
		}
		rep.add(c.Cluster.Name, "cluster", c.Provider, c.Cluster.Environment, c.Cluster.Status, pricing.Cluster(clusterSpecOf(c.Cluster)), c.Cluster.DesiredSpec)
	}

	dbs, err := s.ListDatabases(ctx)
	if err != nil {
		return nil, err
	}
	for _, d := range dbs {
		if d.Resource.Status == "destroyed" {
			continue
		}
		rep.add(d.Resource.Name, "database", d.Provider, d.Resource.Environment, d.Resource.Status, pricing.Database(d.Spec), d.Resource.Spec)
	}

	tables, err := s.ListTables(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tables {
		if t.Resource.Status == "destroyed" {
			continue
		}
		rep.add(t.Resource.Name, "table", t.Provider, t.Resource.Environment, t.Resource.Status, pricing.Table(t.Spec), t.Resource.Spec)
	}

	buckets, err := s.ListS3(ctx)
	if err != nil {
		return nil, err
	}
	for _, b := range buckets {
		if b.Resource.Status == "destroyed" {
			continue
		}
		rep.add(b.Resource.Name, "object-storage", b.Provider, b.Resource.Environment, b.Resource.Status, pricing.S3(b.Spec), b.Resource.Spec)
	}

	secrets, err := s.ListSecrets(ctx)
	if err != nil {
		return nil, err
	}
	for _, sec := range secrets {
		if sec.Resource.Status == "destroyed" {
			continue
		}
		rep.add(sec.Resource.Name, "secret", sec.Provider, sec.Resource.Environment, sec.Resource.Status, pricing.Secret(sec.Spec), sec.Resource.Spec)
	}

	queues, err := s.ListQueues(ctx)
	if err != nil {
		return nil, err
	}
	for _, q := range queues {
		if q.Resource.Status == "destroyed" {
			continue
		}
		rep.add(q.Resource.Name, "queue", q.Provider, q.Resource.Environment, q.Resource.Status, pricing.Queue(q.Spec), q.Resource.Spec)
	}

	functions, err := s.ListFunctions(ctx)
	if err != nil {
		return nil, err
	}
	for _, fn := range functions {
		if fn.Resource.Status == "destroyed" {
			continue
		}
		rep.add(fn.Resource.Name, "function", fn.Provider, fn.Resource.Environment, fn.Resource.Status, pricing.Function(fn.Spec), fn.Resource.Spec)
	}

	stacks, err := s.ListStacks(ctx)
	if err != nil {
		return nil, err
	}
	for _, st := range stacks {
		if st.Resource.Status == "destroyed" {
			continue
		}
		rep.add(st.Resource.Name, "stack", st.Provider, st.Resource.Environment, st.Resource.Status, pricing.Stack(), st.Resource.Spec)
	}

	return rep, nil
}
