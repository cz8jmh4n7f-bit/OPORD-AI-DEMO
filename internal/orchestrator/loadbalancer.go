package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/google/uuid"
)

// CreateLoadBalancerInput is the request to provision a load balancer (ALB).
type CreateLoadBalancerInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.LoadBalancerSpec
	DryRun      bool
}

// CreateLoadBalancerResult reports the outcome (dry-run summary, or persisted resource).
type CreateLoadBalancerResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// LoadBalancerSummary is a load balancer resource enriched for list/detail views.
type LoadBalancerSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.LoadBalancerSpec
}

func loadBalancerSpecOf(r db.Resource) models.LoadBalancerSpec {
	var s models.LoadBalancerSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

func validateLoadBalancerSpec(spec models.LoadBalancerSpec, fallbackName string) error {
	name := spec.Name
	if name == "" {
		name = fallbackName
	}
	if name == "" {
		return fmt.Errorf("invalid loadbalancer spec: name is required")
	}
	return nil
}

// CreateLoadBalancer validates a load balancer spec and (unless DryRun) persists
// it and provisions it in the background. Requires a provider implementing
// LoadBalancerProvisioner.
func (s *Service) CreateLoadBalancer(ctx context.Context, in CreateLoadBalancerInput) (*CreateLoadBalancerResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("loadbalancer name and provider are required")
	}
	if in.Spec.Name == "" {
		in.Spec.Name = in.Name
	}
	if err := validateLoadBalancerSpec(in.Spec, in.Name); err != nil {
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
	lp, ok := prov.(providers.LoadBalancerProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support load balancers", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := lp.PreflightLoadBalancer(ctx, providers.LoadBalancerRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("loadbalancer preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; load balancer %q on %s", in.Name, in.Provider)
		s.log.Info("loadbalancer preflight ok", "name", in.Name, "provider", in.Provider)
		return &CreateLoadBalancerResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling loadbalancer spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "loadbalancer",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating loadbalancer resource: %w", err)
	}
	s.log.Info("loadbalancer resource created", "name", r.Name)
	s.emit("loadbalancer", "created", r.Name, env, in.Provider, in.Spec.Name)
	s.startProvisionLoadBalancer(r.ID)
	return &CreateLoadBalancerResult{Resource: &r}, nil
}

func (s *Service) startProvisionLoadBalancer(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionLoadBalancer(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_loadbalancer failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionLoadBalancerByID(ctx, resourceID)
	}()
}

// ProvisionLoadBalancerByID loads a load balancer resource + its provider and runs
// the tofu apply.
func (s *Service) ProvisionLoadBalancerByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading loadbalancer resource: %w", err)
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
	lp, ok := prov.(providers.LoadBalancerProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support load balancers", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("loadbalancer provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := lp.ProvisionLoadBalancer(ctx, providers.LoadBalancerRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: loadBalancerSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("loadbalancer provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("loadbalancer", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("loadbalancer provisioning complete", "name", r.Name, "dns", res.DNSName)
	s.emit("loadbalancer", "ready", r.Name, r.Environment, p.Name, res.ARN)
	return nil
}

// DestroyLoadBalancer tears down a load balancer resource (tofu destroy) and marks
// it destroyed.
func (s *Service) DestroyLoadBalancer(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("loadbalancer %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	lp, ok := prov.(providers.LoadBalancerProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support load balancers", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("loadbalancer destroy started", "name", r.Name)

	if err := lp.DestroyLoadBalancer(ctx, providers.LoadBalancerRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: loadBalancerSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("loadbalancer destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("loadbalancer destroy complete", "name", r.Name)
	s.emit("loadbalancer", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyLoadBalancerAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyLoadBalancerAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyLoadBalancer(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_loadbalancer failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyLoadBalancer(ctx, name, env); err != nil {
			s.log.Error("async loadbalancer destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteLoadBalancerRecord forgets a terminal load balancer resource's tracking row
// (no tofu).
func (s *Service) DeleteLoadBalancerRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("loadbalancer %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("loadbalancer %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing loadbalancer record %q: %w", name, err)
	}
	s.log.Info("loadbalancer record removed", "name", name)
	return nil
}

// ListLoadBalancers returns all load balancer resources with provider name + parsed spec.
func (s *Service) ListLoadBalancers(ctx context.Context) ([]LoadBalancerSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "loadbalancer")
	if err != nil {
		return nil, fmt.Errorf("listing loadbalancer resources: %w", err)
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
	out := make([]LoadBalancerSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, LoadBalancerSummary{Resource: r, Provider: names[r.ProviderID], Spec: loadBalancerSpecOf(r)})
	}
	return out, nil
}

// LoadBalancerStatus returns one load balancer resource by name + environment.
func (s *Service) LoadBalancerStatus(ctx context.Context, name, env string) (*LoadBalancerSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("loadbalancer %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("loadbalancer %q (env %q) not found", name, env)
	}
	summary := &LoadBalancerSummary{Resource: r, Spec: loadBalancerSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
