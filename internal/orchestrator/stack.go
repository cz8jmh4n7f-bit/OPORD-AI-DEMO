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

// CreateStackInput is the request to provision a generic OpenTofu stack.
type CreateStackInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.StackSpec
	DryRun      bool
}

// CreateStackResult reports the outcome (dry-run summary, or persisted resource).
type CreateStackResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// StackSummary is a stack resource enriched for list/detail views.
type StackSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.StackSpec
}

func stackSpecOf(r db.Resource) models.StackSpec {
	var s models.StackSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

// CreateStack validates a stack spec and (unless DryRun) persists it and runs it
// in the background (tofu apply on the module). Requires a provider that
// implements StackProvisioner (AWS today).
func (s *Service) CreateStack(ctx context.Context, in CreateStackInput) (*CreateStackResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("stack name and provider are required")
	}
	if strings.TrimSpace(in.Spec.ModuleDir) == "" {
		return nil, fmt.Errorf("stack module_dir is required")
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
	sp, ok := prov.(providers.StackProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support generic stacks", p.Type)
	}
	var cfg map[string]any
	_ = json.Unmarshal(p.Config, &cfg)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := sp.PreflightStack(ctx, providers.StackRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("stack preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; module %q (%d var(s)) on %s", in.Spec.ModuleDir, len(in.Spec.Variables), in.Provider)
		s.log.Info("stack preflight ok", "name", in.Name, "provider", in.Provider)
		return &CreateStackResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling stack spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "stack",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating stack resource: %w", err)
	}
	s.log.Info("stack resource created", "name", r.Name, "module", in.Spec.ModuleDir)
	s.emit("stack", "created", r.Name, env, in.Provider, in.Spec.ModuleDir)
	s.startProvisionStack(r.ID)
	return &CreateStackResult{Resource: &r}, nil
}

func (s *Service) startProvisionStack(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionStack(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_stack failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		_ = s.ProvisionStackByID(ctx, resourceID)
	}()
}

// ProvisionStackByID loads a stack resource + its provider and runs tofu apply,
// recording the outputs. Status flows provisioning -> ready/failed.
func (s *Service) ProvisionStackByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading stack resource: %w", err)
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
	sp, ok := prov.(providers.StackProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support generic stacks", p.Type)
	}
	var cfg map[string]any
	_ = json.Unmarshal(p.Config, &cfg)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("stack provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := sp.ProvisionStack(ctx, providers.StackRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: stackSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("stack provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("stack", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("stack provisioning complete", "name", r.Name, "outputs", len(res.Outputs))
	s.emit("stack", "ready", r.Name, r.Environment, p.Name, fmt.Sprintf("%d output(s)", len(res.Outputs)))
	return nil
}

// DestroyStack tears down a stack resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyStack(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("stack %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	sp, ok := prov.(providers.StackProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support generic stacks", p.Type)
	}
	var cfg map[string]any
	_ = json.Unmarshal(p.Config, &cfg)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("stack destroy started", "name", r.Name)

	if err := sp.DestroyStack(ctx, providers.StackRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: stackSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("stack destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("stack destroy complete", "name", r.Name)
	s.emit("stack", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyStackAsync runs DestroyStack on a background context.
// DeleteStackRecord forgets a terminal stack's tracking row (no tofu). Allowed
// only for destroyed/failed - destroy a live stack first. Mirrors DeleteVMRecord.
func (s *Service) DeleteStackRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("stack %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("stack %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing stack record %q: %w", name, err)
	}
	s.log.Info("stack record removed", "name", name)
	return nil
}

func (s *Service) DestroyStackAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyStack(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_stack failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
		defer cancel()
		if err := s.DestroyStack(ctx, name, env); err != nil {
			s.log.Error("async stack destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// ListStacks returns all stack resources with provider name + parsed spec.
func (s *Service) ListStacks(ctx context.Context) ([]StackSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "stack")
	if err != nil {
		return nil, fmt.Errorf("listing stacks: %w", err)
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
	out := make([]StackSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, StackSummary{Resource: r, Provider: names[r.ProviderID], Spec: stackSpecOf(r)})
	}
	return out, nil
}

// StackStatus returns one stack resource by name + environment.
func (s *Service) StackStatus(ctx context.Context, name, env string) (*StackSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("stack %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("stack %q (env %q) not found", name, env)
	}
	summary := &StackSummary{Resource: r, Spec: stackSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
