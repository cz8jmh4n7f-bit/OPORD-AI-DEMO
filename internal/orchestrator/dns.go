package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/google/uuid"
)

// CreateDNSInput is the request to provision a DNS zone.
type CreateDNSInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.DNSSpec
	DryRun      bool
}

// CreateDNSResult reports the outcome (dry-run summary, or persisted resource).
type CreateDNSResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// DNSSummary is a DNS resource enriched for list/detail views.
type DNSSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.DNSSpec
}

func dnsSpecOf(r db.Resource) models.DNSSpec {
	var s models.DNSSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

func validateDNSSpec(spec models.DNSSpec, fallbackName string) error {
	name := spec.Name
	if name == "" {
		name = fallbackName
	}
	var errs []string
	if name == "" {
		errs = append(errs, "name (the domain) is required")
	}
	if spec.Private && spec.VPCID == "" {
		errs = append(errs, "vpc_id is required for a private zone")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid dns spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateDNS validates a DNS spec and (unless DryRun) persists it and provisions
// it in the background. Requires a provider implementing DNSProvisioner.
func (s *Service) CreateDNS(ctx context.Context, in CreateDNSInput) (*CreateDNSResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("dns name and provider are required")
	}
	if in.Spec.Name == "" {
		in.Spec.Name = in.Name
	}
	if err := validateDNSSpec(in.Spec, in.Name); err != nil {
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
	sp, ok := prov.(providers.DNSProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support DNS zones", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := sp.PreflightDNS(ctx, providers.DNSRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("dns preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; DNS zone %q on %s", in.Spec.Name, in.Provider)
		s.log.Info("dns preflight ok", "name", in.Spec.Name, "provider", in.Provider)
		return &CreateDNSResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling dns spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "dns",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating dns resource: %w", err)
	}
	s.log.Info("dns resource created", "name", r.Name, "zone", in.Spec.Name)
	s.emit("dns", "created", r.Name, env, in.Provider, in.Spec.Name)
	s.startProvisionDNS(r.ID)
	return &CreateDNSResult{Resource: &r}, nil
}

func (s *Service) startProvisionDNS(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionDNS(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_dns failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionDNSByID(ctx, resourceID)
	}()
}

// ProvisionDNSByID loads a DNS resource + its provider and runs the tofu apply.
func (s *Service) ProvisionDNSByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading dns resource: %w", err)
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
	sp, ok := prov.(providers.DNSProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support DNS zones", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("dns provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := sp.ProvisionDNS(ctx, providers.DNSRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: dnsSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("dns provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("dns", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("dns provisioning complete", "name", r.Name, "zone", res.ZoneID)
	s.emit("dns", "ready", r.Name, r.Environment, p.Name, res.ZoneName)
	return nil
}

// DestroyDNS tears down a DNS resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyDNS(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("dns %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	sp, ok := prov.(providers.DNSProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support DNS zones", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("dns destroy started", "name", r.Name)

	if err := sp.DestroyDNS(ctx, providers.DNSRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: dnsSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("dns destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("dns destroy complete", "name", r.Name)
	s.emit("dns", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyDNSAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyDNSAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyDNS(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_dns failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyDNS(ctx, name, env); err != nil {
			s.log.Error("async dns destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteDNSRecord forgets a terminal DNS resource's tracking row (no tofu).
func (s *Service) DeleteDNSRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("dns %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("dns %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing dns record %q: %w", name, err)
	}
	s.log.Info("dns record removed", "name", name)
	return nil
}

// ListDNS returns all DNS resources with provider name + parsed spec.
func (s *Service) ListDNS(ctx context.Context) ([]DNSSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "dns")
	if err != nil {
		return nil, fmt.Errorf("listing dns resources: %w", err)
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
	out := make([]DNSSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, DNSSummary{Resource: r, Provider: names[r.ProviderID], Spec: dnsSpecOf(r)})
	}
	return out, nil
}

// DNSStatus returns one DNS resource by name + environment.
func (s *Service) DNSStatus(ctx context.Context, name, env string) (*DNSSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("dns %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("dns %q (env %q) not found", name, env)
	}
	summary := &DNSSummary{Resource: r, Spec: dnsSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
