package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// CreateCDNInput is the request to provision a CloudFront distribution.
type CreateCDNInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.CDNSpec
	DryRun      bool
}

// CreateCDNResult reports the outcome (dry-run summary, or persisted resource).
type CreateCDNResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// CDNSummary is a CDN resource enriched for list/detail views.
type CDNSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.CDNSpec
}

func cdnSpecOf(r db.Resource) models.CDNSpec {
	var s models.CDNSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

func validateCDNSpec(spec models.CDNSpec) error {
	if spec.OriginDomain == "" {
		return fmt.Errorf("invalid cdn spec: origin_domain is required (the origin to serve)")
	}
	return nil
}

// CreateCDN validates a CDN spec and (unless DryRun) persists it and provisions
// it in the background. Requires a provider implementing CDNProvisioner.
func (s *Service) CreateCDN(ctx context.Context, in CreateCDNInput) (*CreateCDNResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("cdn name and provider are required")
	}
	if in.Spec.Name == "" {
		in.Spec.Name = in.Name
	}
	if err := validateCDNSpec(in.Spec); err != nil {
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
	cp, ok := prov.(providers.CDNProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support CDN distributions", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := cp.PreflightCDN(ctx, providers.CDNRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("cdn preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; CDN %q (origin %s) on %s", in.Name, in.Spec.OriginDomain, in.Provider)
		s.log.Info("cdn preflight ok", "name", in.Name, "provider", in.Provider)
		return &CreateCDNResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling cdn spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "cdn",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating cdn resource: %w", err)
	}
	s.log.Info("cdn resource created", "name", r.Name, "origin", in.Spec.OriginDomain)
	s.emit("cdn", "created", r.Name, env, in.Provider, in.Spec.OriginDomain)
	s.startProvisionCDN(r.ID)
	return &CreateCDNResult{Resource: &r}, nil
}

func (s *Service) startProvisionCDN(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionCDN(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_cdn failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionCDNByID(ctx, resourceID)
	}()
}

// ProvisionCDNByID loads a CDN resource + its provider and runs the tofu apply.
func (s *Service) ProvisionCDNByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading cdn resource: %w", err)
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
	cp, ok := prov.(providers.CDNProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support CDN distributions", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("cdn provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := cp.ProvisionCDN(ctx, providers.CDNRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: cdnSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("cdn provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("cdn", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("cdn provisioning complete", "name", r.Name, "distribution", res.DistributionID)
	s.emit("cdn", "ready", r.Name, r.Environment, p.Name, res.DomainName)
	return nil
}

// DestroyCDN tears down a CDN resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyCDN(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("cdn %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	cp, ok := prov.(providers.CDNProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support CDN distributions", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("cdn destroy started", "name", r.Name)

	if err := cp.DestroyCDN(ctx, providers.CDNRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: cdnSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("cdn destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("cdn destroy complete", "name", r.Name)
	s.emit("cdn", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyCDNAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyCDNAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyCDN(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_cdn failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyCDN(ctx, name, env); err != nil {
			s.log.Error("async cdn destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteCDNRecord forgets a terminal CDN resource's tracking row (no tofu).
func (s *Service) DeleteCDNRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("cdn %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("cdn %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing cdn record %q: %w", name, err)
	}
	s.log.Info("cdn record removed", "name", name)
	return nil
}

// ListCDN returns all CDN resources with provider name + parsed spec.
func (s *Service) ListCDN(ctx context.Context) ([]CDNSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "cdn")
	if err != nil {
		return nil, fmt.Errorf("listing cdn resources: %w", err)
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
	out := make([]CDNSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, CDNSummary{Resource: r, Provider: names[r.ProviderID], Spec: cdnSpecOf(r)})
	}
	return out, nil
}

// CDNStatus returns one CDN resource by name + environment.
func (s *Service) CDNStatus(ctx context.Context, name, env string) (*CDNSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("cdn %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("cdn %q (env %q) not found", name, env)
	}
	summary := &CDNSummary{Resource: r, Spec: cdnSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
