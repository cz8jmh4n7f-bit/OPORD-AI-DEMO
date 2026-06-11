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

// CreateCertInput is the request to provision a TLS certificate (ACM).
type CreateCertInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.CertSpec
	DryRun      bool
}

// CreateCertResult reports the outcome (dry-run summary, or persisted resource).
type CreateCertResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// CertSummary is a cert resource enriched for list/detail views.
type CertSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.CertSpec
}

func certSpecOf(r db.Resource) models.CertSpec {
	var s models.CertSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

func validateCertSpec(spec models.CertSpec) error {
	if spec.Domain == "" {
		return fmt.Errorf("invalid cert spec: domain is required")
	}
	return nil
}

// CreateCert validates a cert spec and (unless DryRun) persists it and provisions
// it in the background. Requires a provider implementing CertProvisioner.
func (s *Service) CreateCert(ctx context.Context, in CreateCertInput) (*CreateCertResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("cert name and provider are required")
	}
	if err := validateCertSpec(in.Spec); err != nil {
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
	cp, ok := prov.(providers.CertProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support TLS certificates", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := cp.PreflightCert(ctx, providers.CertRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("cert preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; ACM cert for %q on %s", in.Spec.Domain, in.Provider)
		s.log.Info("cert preflight ok", "name", in.Name, "domain", in.Spec.Domain, "provider", in.Provider)
		return &CreateCertResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling cert spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "cert",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating cert resource: %w", err)
	}
	s.log.Info("cert resource created", "name", r.Name, "domain", in.Spec.Domain)
	s.emit("cert", "created", r.Name, env, in.Provider, in.Spec.Domain)
	s.startProvisionCert(r.ID)
	return &CreateCertResult{Resource: &r}, nil
}

func (s *Service) startProvisionCert(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionCert(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_cert failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionCertByID(ctx, resourceID)
	}()
}

// ProvisionCertByID loads a cert resource + its provider and runs the tofu apply.
func (s *Service) ProvisionCertByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading cert resource: %w", err)
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
	cp, ok := prov.(providers.CertProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support TLS certificates", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("cert provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := cp.ProvisionCert(ctx, providers.CertRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: certSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("cert provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("cert", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("cert provisioning complete", "name", r.Name, "arn", res.ARN)
	s.emit("cert", "ready", r.Name, r.Environment, p.Name, res.ARN)
	return nil
}

// DestroyCert tears down a cert resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyCert(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("cert %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	cp, ok := prov.(providers.CertProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support TLS certificates", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("cert destroy started", "name", r.Name)

	if err := cp.DestroyCert(ctx, providers.CertRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: certSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("cert destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("cert destroy complete", "name", r.Name)
	s.emit("cert", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyCertAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyCertAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyCert(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_cert failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyCert(ctx, name, env); err != nil {
			s.log.Error("async cert destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteCertRecord forgets a terminal cert resource's tracking row (no tofu).
func (s *Service) DeleteCertRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("cert %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("cert %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing cert record %q: %w", name, err)
	}
	s.log.Info("cert record removed", "name", name)
	return nil
}

// ListCert returns all cert resources with provider name + parsed spec.
func (s *Service) ListCert(ctx context.Context) ([]CertSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "cert")
	if err != nil {
		return nil, fmt.Errorf("listing cert resources: %w", err)
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
	out := make([]CertSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, CertSummary{Resource: r, Provider: names[r.ProviderID], Spec: certSpecOf(r)})
	}
	return out, nil
}

// CertStatus returns one cert resource by name + environment.
func (s *Service) CertStatus(ctx context.Context, name, env string) (*CertSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("cert %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("cert %q (env %q) not found", name, env)
	}
	summary := &CertSummary{Resource: r, Spec: certSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
