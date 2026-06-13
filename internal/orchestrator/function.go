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

// CreateFunctionInput is the request to provision a serverless function (Lambda).
type CreateFunctionInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.FunctionSpec
	DryRun      bool
}

// CreateFunctionResult reports the outcome (dry-run summary, or persisted resource).
type CreateFunctionResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// FunctionSummary is a function resource enriched for list/detail views.
type FunctionSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.FunctionSpec
}

func functionSpecOf(r db.Resource) models.FunctionSpec {
	var s models.FunctionSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

func validateFunctionSpec(spec models.FunctionSpec) error {
	var errs []string
	if spec.MemoryMB != 0 && (spec.MemoryMB < 128 || spec.MemoryMB > 10240) {
		errs = append(errs, "memory_mb must be between 128 and 10240")
	}
	if spec.TimeoutSec != 0 && (spec.TimeoutSec < 1 || spec.TimeoutSec > 900) {
		errs = append(errs, "timeout_sec must be between 1 and 900")
	}
	if (spec.S3Bucket == "") != (spec.S3Key == "") {
		errs = append(errs, "s3_bucket and s3_key must be set together (or both empty for the built-in handler)")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid function spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateFunction validates a function spec and (unless DryRun) persists it and
// provisions it in the background. Requires a provider implementing FunctionProvisioner.
func (s *Service) CreateFunction(ctx context.Context, in CreateFunctionInput) (*CreateFunctionResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("function name and provider are required")
	}
	if err := validateFunctionSpec(in.Spec); err != nil {
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
	fp, ok := prov.(providers.FunctionProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support serverless functions", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := fp.PreflightFunction(ctx, providers.FunctionRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("function preflight failed: %w", err)
	}

	if in.DryRun {
		runtime := in.Spec.Runtime
		if runtime == "" {
			runtime = "python3.12"
		}
		code := "built-in handler"
		if in.Spec.S3Bucket != "" {
			code = "s3://" + in.Spec.S3Bucket + "/" + in.Spec.S3Key
		}
		summary := fmt.Sprintf("spec valid; function %q (%s, %s) on %s", in.Name, runtime, code, in.Provider)
		s.log.Info("function preflight ok", "name", in.Name, "provider", in.Provider)
		return &CreateFunctionResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling function spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "function",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating function resource: %w", err)
	}
	s.log.Info("function resource created", "name", r.Name, "runtime", in.Spec.Runtime)
	s.emit("function", "created", r.Name, env, in.Provider, in.Spec.Runtime)
	s.startProvisionFunction(r.ID)
	return &CreateFunctionResult{Resource: &r}, nil
}

func (s *Service) startProvisionFunction(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionFunction(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_function failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionFunctionByID(ctx, resourceID)
	}()
}

// ProvisionFunctionByID loads a function resource + its provider and runs the
// tofu apply, recording the outcome. Status flows provisioning -> ready/failed.
func (s *Service) ProvisionFunctionByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading function resource: %w", err)
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
	fp, ok := prov.(providers.FunctionProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support serverless functions", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveFunctionCreds(ctx, p)

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("function provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := fp.ProvisionFunction(ctx, providers.FunctionRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: functionSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("function provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("function", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("function provisioning complete", "name", r.Name, "arn", res.ARN)
	s.emit("function", "ready", r.Name, r.Environment, p.Name, res.ARN)
	return nil
}

// DestroyFunction tears down a function resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyFunction(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("function %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	fp, ok := prov.(providers.FunctionProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support serverless functions", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveFunctionCreds(ctx, p)

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("function destroy started", "name", r.Name)

	if err := fp.DestroyFunction(ctx, providers.FunctionRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: functionSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("function destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("function destroy complete", "name", r.Name)
	s.emit("function", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyFunctionAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyFunctionAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyFunction(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_function failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyFunction(ctx, name, env); err != nil {
			s.log.Error("async function destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteFunctionRecord forgets a terminal function's tracking row (no tofu).
// Allowed only for destroyed/failed - destroy a live function first.
func (s *Service) DeleteFunctionRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("function %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("function %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing function record %q: %w", name, err)
	}
	s.log.Info("function record removed", "name", name)
	return nil
}

// ListFunctions returns all function resources with provider name + parsed spec.
func (s *Service) ListFunctions(ctx context.Context) ([]FunctionSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "function")
	if err != nil {
		return nil, fmt.Errorf("listing functions: %w", err)
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
	out := make([]FunctionSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, FunctionSummary{Resource: r, Provider: names[r.ProviderID], Spec: functionSpecOf(r)})
	}
	return out, nil
}

// FunctionStatus returns one function resource by name + environment.
func (s *Service) FunctionStatus(ctx context.Context, name, env string) (*FunctionSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("function %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("function %q (env %q) not found", name, env)
	}
	summary := &FunctionSummary{Resource: r, Spec: functionSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
