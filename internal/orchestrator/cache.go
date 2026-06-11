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

// CreateCacheInput is the request to provision an in-memory cache.
type CreateCacheInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.CacheSpec
	DryRun      bool
}

// CreateCacheResult reports the outcome (dry-run summary, or persisted resource).
type CreateCacheResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// CacheSummary is a cache resource enriched for list/detail views.
type CacheSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.CacheSpec
}

func cacheSpecOf(r db.Resource) models.CacheSpec {
	var s models.CacheSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

// Cache names: AWS replication group ids are 1-40 lowercase letters/numbers/hyphens.
// Azure Redis names are sanitised by the provider before apply.
var cacheNameRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

func validateCacheSpec(spec models.CacheSpec, fallbackName string) error {
	name := spec.Name
	if name == "" {
		name = fallbackName
	}
	var errs []string
	if !cacheNameRe.MatchString(name) {
		errs = append(errs, "name must be 1-40 chars: lowercase letters, numbers, hyphens, starting with a letter")
	}
	if spec.NumCacheNodes < 0 {
		errs = append(errs, "num_cache_nodes must be >= 0")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid cache spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateCache validates a cache spec and (unless DryRun) persists it and
// provisions it in the background. Requires a provider implementing CacheProvisioner.
func (s *Service) CreateCache(ctx context.Context, in CreateCacheInput) (*CreateCacheResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("cache name and provider are required")
	}
	if in.Spec.Name == "" {
		in.Spec.Name = in.Name
	}
	if err := validateCacheSpec(in.Spec, in.Name); err != nil {
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
	cp, ok := prov.(providers.CacheProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support managed caches", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := cp.PreflightCache(ctx, providers.CacheRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("cache preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; managed Redis cache %q on %s", in.Spec.Name, in.Provider)
		s.log.Info("cache preflight ok", "name", in.Spec.Name, "provider", in.Provider)
		return &CreateCacheResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling cache spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "cache",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating cache resource: %w", err)
	}
	s.log.Info("cache resource created", "name", r.Name, "cache", in.Spec.Name)
	s.emit("cache", "created", r.Name, env, in.Provider, in.Spec.Name)
	s.startProvisionCache(r.ID)
	return &CreateCacheResult{Resource: &r}, nil
}

func (s *Service) startProvisionCache(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionCache(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_cache failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionCacheByID(ctx, resourceID)
	}()
}

// ProvisionCacheByID loads a cache resource + its provider and runs tofu apply.
func (s *Service) ProvisionCacheByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading cache resource: %w", err)
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
	cp, ok := prov.(providers.CacheProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support managed caches", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("cache provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := cp.ProvisionCache(ctx, providers.CacheRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: cacheSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("cache provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("cache", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("cache provisioning complete", "name", r.Name, "endpoint", res.PrimaryEndpoint)
	s.emit("cache", "ready", r.Name, r.Environment, p.Name, res.PrimaryEndpoint)
	return nil
}

// DestroyCache tears down a cache resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyCache(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("cache %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	cp, ok := prov.(providers.CacheProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support managed caches", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("cache destroy started", "name", r.Name)

	if err := cp.DestroyCache(ctx, providers.CacheRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: cacheSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("cache destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("cache destroy complete", "name", r.Name)
	s.emit("cache", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyCacheAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyCacheAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyCache(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_cache failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyCache(ctx, name, env); err != nil {
			s.log.Error("async cache destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteCacheRecord forgets a terminal cache resource's tracking row (no tofu).
func (s *Service) DeleteCacheRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("cache %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("cache %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing cache record %q: %w", name, err)
	}
	s.log.Info("cache record removed", "name", name)
	return nil
}

// ListCaches returns all cache resources with provider name + parsed spec.
func (s *Service) ListCaches(ctx context.Context) ([]CacheSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "cache")
	if err != nil {
		return nil, fmt.Errorf("listing cache resources: %w", err)
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
	out := make([]CacheSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, CacheSummary{Resource: r, Provider: names[r.ProviderID], Spec: cacheSpecOf(r)})
	}
	return out, nil
}

// CacheStatus returns one cache resource by name + environment.
func (s *Service) CacheStatus(ctx context.Context, name, env string) (*CacheSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("cache %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("cache %q (env %q) not found", name, env)
	}
	summary := &CacheSummary{Resource: r, Spec: cacheSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
