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

// CreateVMInput is the structured request to provision standalone VMs.
type CreateVMInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.VMSpec
	DryRun      bool
}

// CreateVMResult reports the outcome of CreateVM. For a dry run, Summary is set
// and nothing was persisted; otherwise Resource identifies the new record.
type CreateVMResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// VMSummary is a vm resource enriched for list/detail views.
type VMSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.VMSpec
}

func vmSpecOf(r db.Resource) models.VMSpec {
	var s models.VMSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

// CreateVM validates a VM spec and (unless DryRun) persists it as a pending
// resource, then provisions it in the background (Tofu apply). A dry run runs
// the same spec validation + provider preflight offline and persists nothing.
func (s *Service) CreateVM(ctx context.Context, in CreateVMInput) (*CreateVMResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("vm name and provider are required")
	}
	env := in.Environment
	if env == "" {
		env = "dev"
	}

	p, err := s.q.GetProviderByName(ctx, in.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found (register it with `opord provider add`): %w", in.Provider, err)
	}

	var errs []string
	// template = image identifier. AWS (AMI) and on-prem (golden image) need it;
	// Azure and GCP tofu modules ship a default Ubuntu 22.04 LTS image, so
	// template is optional there.
	if strings.TrimSpace(in.Spec.Template) == "" &&
		p.Type != string(models.ProviderAzure) && p.Type != string(models.ProviderGCP) {
		errs = append(errs, "template is required")
	}
	if in.Spec.Count < 1 {
		errs = append(errs, "count must be >= 1")
	}
	if in.Spec.DiskGB < 1 {
		errs = append(errs, "disk must be >= 1 GB")
	}
	// On-prem sizing is by cpu/memory; cloud sizing is by instance_type.
	if in.Spec.InstanceType == "" {
		if in.Spec.CPU < 1 {
			errs = append(errs, "cpu (or instance_type) is required")
		}
		if in.Spec.MemoryMB < 512 {
			errs = append(errs, "memory_mb >= 512 (or instance_type) is required")
		}
	}
	// Azure VMs are SSH-only - no password auth in the module.
	if p.Type == string(models.ProviderAzure) && strings.TrimSpace(in.Spec.SSHPublicKey) == "" {
		errs = append(errs, "ssh_public_key is required for Azure VMs (no password auth)")
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid vm spec: %s", strings.Join(errs, "; "))
	}

	if in.Spec.NamePrefix == "" {
		in.Spec.NamePrefix = in.Name
	}
	if in.Spec.DNSSuffix == "" {
		in.Spec.DNSSuffix = "local"
	}

	// Resolve the provider plugin + creds + config once (preflight + provision).
	var (
		vmp   providers.VMProvisioner
		cfg   map[string]any
		creds map[string]string
	)
	if prov, gerr := s.registry.Get(models.ProviderType(p.Type)); gerr == nil {
		if vp, ok := prov.(providers.VMProvisioner); ok {
			vmp = vp
			cfg = s.providerCfg(ctx, p)
			creds, _ = s.creds.Resolve(ctx, p)
			if err := vp.PreflightVM(ctx, providers.VMRequest{
				Workspace: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
			}); err != nil {
				return nil, fmt.Errorf("vm preflight failed: %w", err)
			}
		}
	}

	if in.DryRun {
		sizing := fmt.Sprintf("%d vCPU / %d MB", in.Spec.CPU, in.Spec.MemoryMB)
		if in.Spec.InstanceType != "" {
			sizing = in.Spec.InstanceType
		}
		summary := fmt.Sprintf("spec valid; %d × %q (%s, %d GB disk) on %s",
			in.Spec.Count, in.Spec.Template, sizing, in.Spec.DiskGB, in.Provider)
		s.log.Info("vm preflight ok", "name", in.Name, "provider", in.Provider)
		return &CreateVMResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling vm spec: %w", err)
	}

	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "vm",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating vm resource: %w", err)
	}
	s.log.Info("vm resource created", "name", r.Name, "count", in.Spec.Count)
	s.emit("vm", "created", r.Name, env, in.Provider, fmt.Sprintf("%d × %q", in.Spec.Count, in.Spec.Template))

	// Provision in the background (tofu apply can take minutes): status flows
	// pending -> provisioning -> ready/failed; the caller returns immediately.
	if vmp != nil {
		s.startProvisionVM(r.ID)
	}
	return &CreateVMResult{Resource: &r}, nil
}

// startProvisionVM hands provisioning to the job queue when configured,
// otherwise runs it in-process. Both paths converge on ProvisionVMByID.
func (s *Service) startProvisionVM(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionVM(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_vm failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionVMByID(ctx, resourceID)
	}()
}

// ProvisionVMByID loads a vm resource plus its provider from the database and
// runs the real Tofu apply, recording the outcome. Everything is loaded by id
// so it is safe to run from either an in-process goroutine or a River worker.
// Status flows provisioning -> ready/failed.
func (s *Service) ProvisionVMByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading vm resource: %w", err)
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
	vmp, ok := prov.(providers.VMProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support vm provisioning", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("vm provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := vmp.ProvisionVM(ctx, providers.VMRequest{
		Workspace: r.TofuWorkspace, Spec: vmSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("vm provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("vm", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("vm provisioning complete", "name", r.Name, "ids", res.IDs, "ips", res.PublicIPs)
	s.emit("vm", "ready", r.Name, r.Environment, p.Name, fmt.Sprintf("ids=%v ips=%v", res.IDs, res.PublicIPs))
	return nil
}

// markVMFailed flips a resource to status="failed". Finding E: when a provision
// cause is supplied, the error reason is persisted into observed (`{"error": …}`)
// so the web/API can surface WHY it failed, instead of a bare status="failed".
// Variadic + backward-compatible: callers without a cause keep the old behavior.
func (s *Service) markVMFailed(ctx context.Context, id uuid.UUID, cause ...error) {
	if len(cause) > 0 && cause[0] != nil {
		if obs, err := json.Marshal(map[string]string{"error": cause[0].Error()}); err == nil {
			_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: id, Observed: obs, Status: "failed"})
			return
		}
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: id, Status: "failed"})
}

// DestroyVM tears down a vm resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyVM(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("vm %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	vp, ok := prov.(providers.VMProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support vm destroy", p.Type)
	}

	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("vm destroy started", "name", r.Name)

	if err := vp.DestroyVM(ctx, providers.VMRequest{
		Workspace: r.TofuWorkspace, Spec: vmSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "failed"})
		return fmt.Errorf("vm destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("vm destroy complete", "name", r.Name)
	s.emit("vm", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DeleteVMRecord removes a VM's tracking row from OPORD. Allowed only for
// terminal states (destroyed/failed) so a running VM is never orphaned - destroy
// it first. Touches no infrastructure; it just forgets the record.
func (s *Service) DeleteVMRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("vm %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
		// terminal - safe to forget
	default:
		return fmt.Errorf("vm %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing vm record %q: %w", name, err)
	}
	s.log.Info("vm record removed", "name", name)
	return nil
}

// DestroyVMAsync enqueues a destroy job when a queue is configured, otherwise
// runs DestroyVM in-process. Used by the HTTP API so the request returns
// immediately; progress shows via the resource status (destroying ->
// destroyed/failed).
func (s *Service) DestroyVMAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyVM(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_vm failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyVM(ctx, name, env); err != nil {
			s.log.Error("async vm destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// ReapExpiredVMs destroys vm resources whose TTL has elapsed (safety net).
func (s *Service) ReapExpiredVMs(ctx context.Context) (int, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "vm")
	if err != nil {
		return 0, err
	}
	destroyed := 0
	for _, r := range rs {
		if r.Status != "ready" && r.Status != "provisioning" {
			continue
		}
		spec := vmSpecOf(r)
		if spec.TTLHours <= 0 {
			continue
		}
		if time.Now().Before(r.CreatedAt.Add(time.Duration(spec.TTLHours) * time.Hour)) {
			continue
		}
		s.log.Info("vm TTL expired - auto-destroying", "name", r.Name, "ttl_hours", spec.TTLHours)
		if err := s.DestroyVM(ctx, r.Name, r.Environment); err != nil {
			s.log.Error("TTL auto-destroy failed", "name", r.Name, "err", err)
			continue
		}
		destroyed++
	}
	return destroyed, nil
}

// ListVMs returns all vm resources with provider name and parsed spec.
func (s *Service) ListVMs(ctx context.Context) ([]VMSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "vm")
	if err != nil {
		return nil, fmt.Errorf("listing vms: %w", err)
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
	out := make([]VMSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, VMSummary{Resource: r, Provider: names[r.ProviderID], Spec: vmSpecOf(r)})
	}
	return out, nil
}

// ScaleVM changes a VM resource's count and re-provisions (tofu apply
// reconciles to the new count - idempotent). A day-2 operation.
func (s *Service) ScaleVM(ctx context.Context, name, env string, count int) error {
	if env == "" {
		env = "dev"
	}
	if count < 1 {
		return fmt.Errorf("count must be >= 1")
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("vm %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return fmt.Errorf("vm %q (env %q) not found", name, env)
	}
	spec := vmSpecOf(r)
	if spec.Count == count {
		return nil
	}
	spec.Count = count
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling vm spec: %w", err)
	}
	if _, err := s.q.UpdateResourceSpec(ctx, db.UpdateResourceSpecParams{ID: r.ID, Spec: specJSON, Status: "provisioning"}); err != nil {
		return fmt.Errorf("updating vm spec: %w", err)
	}
	s.log.Info("vm scaling", "name", r.Name, "count", count)
	s.emit("vm", "scaling", r.Name, r.Environment, "", fmt.Sprintf("count to %d", count))
	s.startProvisionVM(r.ID)
	return nil
}

// VMStatus returns one vm resource by name + environment.
func (s *Service) VMStatus(ctx context.Context, name, env string) (*VMSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("vm %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("vm %q (env %q) not found", name, env)
	}
	summary := &VMSummary{Resource: r, Spec: vmSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
