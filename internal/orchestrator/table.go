package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// CreateTableInput is the request to provision a managed table (DynamoDB).
type CreateTableInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.TableSpec
	DryRun      bool
}

// CreateTableResult reports the outcome (dry-run summary, or persisted resource).
type CreateTableResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// TableSummary is a table resource enriched for list/detail views.
type TableSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.TableSpec
}

func tableSpecOf(r db.Resource) models.TableSpec {
	var s models.TableSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

func validateTableSpec(spec models.TableSpec) error {
	var errs []string
	if strings.TrimSpace(spec.HashKey) == "" {
		errs = append(errs, "hash_key is required")
	}
	validType := func(t string) bool { return t == "" || t == "S" || t == "N" || t == "B" }
	if !validType(spec.HashKeyType) {
		errs = append(errs, "hash_key_type must be S, N, or B")
	}
	if spec.RangeKey != "" && !validType(spec.RangeKeyType) {
		errs = append(errs, "range_key_type must be S, N, or B")
	}
	switch spec.BillingMode {
	case "", "PAY_PER_REQUEST", "PROVISIONED":
	default:
		errs = append(errs, "billing_mode must be PAY_PER_REQUEST or PROVISIONED")
	}
	if spec.BillingMode == "PROVISIONED" && (spec.ReadCapacity < 1 || spec.WriteCapacity < 1) {
		errs = append(errs, "read_capacity and write_capacity must be >= 1 for PROVISIONED")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid table spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateTable validates a table spec and (unless DryRun) persists it and
// provisions it in the background. Requires a provider implementing TableProvisioner.
func (s *Service) CreateTable(ctx context.Context, in CreateTableInput) (*CreateTableResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("table name and provider are required")
	}
	if err := validateTableSpec(in.Spec); err != nil {
		return nil, err
	}
	env := in.Environment
	if env == "" {
		env = "dev"
	}

	p, err := s.q.GetProviderByName(ctx, in.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found (register it with `opord provider add`): %w", in.Provider, err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return nil, err
	}
	tp, ok := prov.(providers.TableProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support managed tables", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := tp.PreflightTable(ctx, providers.TableRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("table preflight failed: %w", err)
	}

	if in.DryRun {
		billing := in.Spec.BillingMode
		if billing == "" {
			billing = "PAY_PER_REQUEST"
		}
		summary := fmt.Sprintf("spec valid; table %q (hash=%s, %s) on %s", in.Name, in.Spec.HashKey, billing, in.Provider)
		s.log.Info("table preflight ok", "name", in.Name, "provider", in.Provider)
		return &CreateTableResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling table spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "table",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating table resource: %w", err)
	}
	s.log.Info("table resource created", "name", r.Name, "hash_key", in.Spec.HashKey)
	s.emit("table", "created", r.Name, env, in.Provider, fmt.Sprintf("hash=%s", in.Spec.HashKey))
	s.startProvisionTable(r.ID)
	return &CreateTableResult{Resource: &r}, nil
}

func (s *Service) startProvisionTable(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionTable(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_table failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionTableByID(ctx, resourceID)
	}()
}

// ProvisionTableByID loads a table resource + its provider and runs the tofu
// apply, recording the outcome. Status flows provisioning -> ready/failed.
func (s *Service) ProvisionTableByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading table resource: %w", err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		s.markVMFailed(ctx, r.ID)
		return err
	}
	tp, ok := prov.(providers.TableProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support managed tables", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("table provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := tp.ProvisionTable(ctx, providers.TableRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: tableSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("table provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("table", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("table provisioning complete", "name", r.Name, "arn", res.ARN)
	s.emit("table", "ready", r.Name, r.Environment, p.Name, res.ARN)
	return nil
}

// DestroyTable tears down a table resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyTable(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("table %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	tp, ok := prov.(providers.TableProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support managed tables", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("table destroy started", "name", r.Name)

	if err := tp.DestroyTable(ctx, providers.TableRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: tableSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("table destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("table destroy complete", "name", r.Name)
	s.emit("table", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyTableAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyTableAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyTable(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_table failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyTable(ctx, name, env); err != nil {
			s.log.Error("async table destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteTableRecord forgets a terminal table's tracking row (no tofu). Allowed
// only for destroyed/failed - destroy a live table first.
func (s *Service) DeleteTableRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("table %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("table %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing table record %q: %w", name, err)
	}
	s.log.Info("table record removed", "name", name)
	return nil
}

// ListTables returns all table resources with provider name + parsed spec.
func (s *Service) ListTables(ctx context.Context) ([]TableSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "table")
	if err != nil {
		return nil, fmt.Errorf("listing tables: %w", err)
	}
	provs, err := s.q.ListProviders(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing providers: %w", err)
	}
	names := make(map[uuid.UUID]string, len(provs))
	for _, p := range provs {
		names[p.ID] = p.Name
	}
	tid, scoped := scopeTenant(ctx)
	out := make([]TableSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, TableSummary{Resource: r, Provider: names[r.ProviderID], Spec: tableSpecOf(r)})
	}
	return out, nil
}

// TableStatus returns one table resource by name + environment.
func (s *Service) TableStatus(ctx context.Context, name, env string) (*TableSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("table %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("table %q (env %q) not found", name, env)
	}
	summary := &TableSummary{Resource: r, Spec: tableSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
