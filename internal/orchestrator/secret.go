package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/google/uuid"
)

// CreateSecretInput is the request to provision a managed secret.
type CreateSecretInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.SecretSpec
	DryRun      bool
}

// CreateSecretResult reports the outcome (dry-run summary, or persisted resource).
type CreateSecretResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// SecretSummary is a secret resource enriched for list/detail views.
type SecretSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.SecretSpec
}

func secretSpecOf(r db.Resource) models.SecretSpec {
	var s models.SecretSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

// Secret names are path-like (AWS Secrets Manager allows /_+=.@-); Azure Key
// Vault prefixes are stricter but the provider sanitises before apply.
var secretNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9/_+=.@-]{0,511}$`)

func validateSecretSpec(spec models.SecretSpec, fallbackName string) error {
	name := spec.Name
	if name == "" {
		name = fallbackName
	}
	var errs []string
	if !secretNameRe.MatchString(name) {
		errs = append(errs, "name must be 1-512 chars of letters, numbers, or /_+=.@- (path-like)")
	}
	if spec.RecoveryWindowDays < 0 {
		errs = append(errs, "recovery_window_days must be >= 0")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid secret spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateSecret validates a secret spec and (unless DryRun) persists it and
// provisions it in the background. Requires a provider implementing
// SecretProvisioner.
func (s *Service) CreateSecret(ctx context.Context, in CreateSecretInput) (*CreateSecretResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("secret name and provider are required")
	}
	if in.Spec.Name == "" {
		in.Spec.Name = in.Name
	}
	if err := validateSecretSpec(in.Spec, in.Name); err != nil {
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
	sp, ok := prov.(providers.SecretProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support managed secrets", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := sp.PreflightSecret(ctx, providers.SecretRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("secret preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; managed secret %q on %s", in.Spec.Name, in.Provider)
		s.log.Info("secret preflight ok", "name", in.Spec.Name, "provider", in.Provider)
		return &CreateSecretResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling secret spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "secret",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating secret resource: %w", err)
	}
	s.log.Info("secret resource created", "name", r.Name, "secret", in.Spec.Name)
	s.emit("secret", "created", r.Name, env, in.Provider, in.Spec.Name)
	s.startProvisionSecret(r.ID)
	return &CreateSecretResult{Resource: &r}, nil
}

func (s *Service) startProvisionSecret(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionSecret(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_secret failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionSecretByID(ctx, resourceID)
	}()
}

// ProvisionSecretByID loads a secret resource + its provider and runs tofu apply.
func (s *Service) ProvisionSecretByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading secret resource: %w", err)
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
	sp, ok := prov.(providers.SecretProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support managed secrets", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("secret provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := sp.ProvisionSecret(ctx, providers.SecretRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: secretSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("secret provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("secret", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("secret provisioning complete", "name", r.Name, "secret", res.SecretID)
	s.emit("secret", "ready", r.Name, r.Environment, p.Name, res.SecretARN)
	return nil
}

// DestroySecret tears down a secret resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroySecret(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("secret %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	sp, ok := prov.(providers.SecretProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support managed secrets", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("secret destroy started", "name", r.Name)

	if err := sp.DestroySecret(ctx, providers.SecretRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: secretSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("secret destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("secret destroy complete", "name", r.Name)
	s.emit("secret", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroySecretAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroySecretAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroySecret(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_secret failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroySecret(ctx, name, env); err != nil {
			s.log.Error("async secret destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteSecretRecord forgets a terminal secret resource's tracking row (no tofu).
func (s *Service) DeleteSecretRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("secret %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("secret %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing secret record %q: %w", name, err)
	}
	s.log.Info("secret record removed", "name", name)
	return nil
}

// ListSecrets returns all secret resources with provider name + parsed spec.
func (s *Service) ListSecrets(ctx context.Context) ([]SecretSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "secret")
	if err != nil {
		return nil, fmt.Errorf("listing secret resources: %w", err)
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
	out := make([]SecretSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, SecretSummary{Resource: r, Provider: names[r.ProviderID], Spec: secretSpecOf(r)})
	}
	return out, nil
}

// SecretStatus returns one secret resource by name + environment.
func (s *Service) SecretStatus(ctx context.Context, name, env string) (*SecretSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("secret %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("secret %q (env %q) not found", name, env)
	}
	summary := &SecretSummary{Resource: r, Spec: secretSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
