package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// CreateS3Input is the request to provision an object storage bucket.
type CreateS3Input struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.S3Spec
	DryRun      bool
}

// CreateS3Result reports the outcome (dry-run summary, or persisted resource).
type CreateS3Result struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// S3Summary is an S3 resource enriched for list/detail views.
type S3Summary struct {
	Resource db.Resource
	Provider string
	Spec     models.S3Spec
}

func s3SpecOf(r db.Resource) models.S3Spec {
	var s models.S3Spec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

var s3NameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)

func validateS3Spec(spec models.S3Spec, fallbackName string) error {
	name := spec.Name
	if name == "" {
		name = fallbackName
	}
	var errs []string
	if !s3NameRe.MatchString(name) || strings.Contains(name, "..") || strings.Contains(name, ".-") || strings.Contains(name, "-.") {
		errs = append(errs, "name must be a valid S3 bucket name (3-63 lowercase letters, numbers, dots, hyphens)")
	}
	if spec.LifecycleGlacierDays < 0 {
		errs = append(errs, "lifecycle_glacier_days must be >= 0")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid s3 spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateS3 validates an S3 spec and (unless DryRun) persists it and provisions
// it in the background. Requires a provider implementing S3Provisioner.
func (s *Service) CreateS3(ctx context.Context, in CreateS3Input) (*CreateS3Result, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("s3 name and provider are required")
	}
	if in.Spec.Name == "" {
		in.Spec.Name = in.Name
	}
	if err := validateS3Spec(in.Spec, in.Name); err != nil {
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
	sp, ok := prov.(providers.S3Provisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support object storage buckets", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := sp.PreflightS3(ctx, providers.S3Request{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("s3 preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; private versioned S3 bucket %q on %s", in.Spec.Name, in.Provider)
		s.log.Info("s3 preflight ok", "name", in.Spec.Name, "provider", in.Provider)
		return &CreateS3Result{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling s3 spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "s3",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating s3 resource: %w", err)
	}
	s.log.Info("s3 resource created", "name", r.Name, "bucket", in.Spec.Name)
	s.emit("s3", "created", r.Name, env, in.Provider, in.Spec.Name)
	s.startProvisionS3(r.ID)
	return &CreateS3Result{Resource: &r}, nil
}

func (s *Service) startProvisionS3(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionS3(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_s3 failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionS3ByID(ctx, resourceID)
	}()
}

// ProvisionS3ByID loads an S3 resource + its provider and runs the tofu apply.
func (s *Service) ProvisionS3ByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading s3 resource: %w", err)
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
	sp, ok := prov.(providers.S3Provisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support object storage buckets", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("s3 provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := sp.ProvisionS3(ctx, providers.S3Request{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: s3SpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("s3 provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("s3", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("s3 provisioning complete", "name", r.Name, "bucket", res.BucketID)
	s.emit("s3", "ready", r.Name, r.Environment, p.Name, res.BucketARN)
	return nil
}

// DestroyS3 tears down an S3 resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyS3(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("s3 %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	sp, ok := prov.(providers.S3Provisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support object storage buckets", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("s3 destroy started", "name", r.Name)

	if err := sp.DestroyS3(ctx, providers.S3Request{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: s3SpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("s3 destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("s3 destroy complete", "name", r.Name)
	s.emit("s3", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyS3Async enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyS3Async(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyS3(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_s3 failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyS3(ctx, name, env); err != nil {
			s.log.Error("async s3 destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteS3Record forgets a terminal S3 resource's tracking row (no tofu).
func (s *Service) DeleteS3Record(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("s3 %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("s3 %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing s3 record %q: %w", name, err)
	}
	s.log.Info("s3 record removed", "name", name)
	return nil
}

// ListS3 returns all S3 resources with provider name + parsed spec.
func (s *Service) ListS3(ctx context.Context) ([]S3Summary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "s3")
	if err != nil {
		return nil, fmt.Errorf("listing s3 resources: %w", err)
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
	out := make([]S3Summary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, S3Summary{Resource: r, Provider: names[r.ProviderID], Spec: s3SpecOf(r)})
	}
	return out, nil
}

// S3Status returns one S3 resource by name + environment.
func (s *Service) S3Status(ctx context.Context, name, env string) (*S3Summary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("s3 %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("s3 %q (env %q) not found", name, env)
	}
	summary := &S3Summary{Resource: r, Spec: s3SpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
