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

// CreateAPIGatewayInput is the request to provision an API gateway.
type CreateAPIGatewayInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.APIGatewaySpec
	DryRun      bool
}

// CreateAPIGatewayResult reports the outcome (dry-run summary, or persisted resource).
type CreateAPIGatewayResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// APIGatewaySummary is an API gateway resource enriched for list/detail views.
type APIGatewaySummary struct {
	Resource db.Resource
	Provider string
	Spec     models.APIGatewaySpec
}

func apiGatewaySpecOf(r db.Resource) models.APIGatewaySpec {
	var s models.APIGatewaySpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

func validateAPIGatewaySpec(spec models.APIGatewaySpec, fallbackName string) error {
	name := spec.Name
	if name == "" {
		name = fallbackName
	}
	var errs []string
	if name == "" {
		errs = append(errs, "name is required")
	}
	switch spec.IntegrationType {
	case "lambda", "http":
		if spec.IntegrationTarget == "" {
			errs = append(errs, "integration_target required")
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid apigateway spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateAPIGateway validates an API gateway spec and (unless DryRun) persists it
// and provisions it in the background. Requires a provider implementing
// APIGatewayProvisioner.
func (s *Service) CreateAPIGateway(ctx context.Context, in CreateAPIGatewayInput) (*CreateAPIGatewayResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("apigateway name and provider are required")
	}
	if in.Spec.Name == "" {
		in.Spec.Name = in.Name
	}
	if err := validateAPIGatewaySpec(in.Spec, in.Name); err != nil {
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
	gp, ok := prov.(providers.APIGatewayProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support API gateways", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := gp.PreflightAPIGateway(ctx, providers.APIGatewayRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("apigateway preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; API Gateway %q on %s", in.Name, in.Provider)
		s.log.Info("apigateway preflight ok", "name", in.Name, "provider", in.Provider)
		return &CreateAPIGatewayResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling apigateway spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "apigateway",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating apigateway resource: %w", err)
	}
	s.log.Info("apigateway resource created", "name", r.Name)
	s.emit("apigateway", "created", r.Name, env, in.Provider, in.Spec.Name)
	s.startProvisionAPIGateway(r.ID)
	return &CreateAPIGatewayResult{Resource: &r}, nil
}

func (s *Service) startProvisionAPIGateway(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionAPIGateway(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_apigateway failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionAPIGatewayByID(ctx, resourceID)
	}()
}

// ProvisionAPIGatewayByID loads an API gateway resource + its provider and runs
// the tofu apply.
func (s *Service) ProvisionAPIGatewayByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading apigateway resource: %w", err)
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
	gp, ok := prov.(providers.APIGatewayProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support API gateways", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("apigateway provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := gp.ProvisionAPIGateway(ctx, providers.APIGatewayRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: apiGatewaySpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("apigateway provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("apigateway", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("apigateway provisioning complete", "name", r.Name, "endpoint", res.Endpoint)
	s.emit("apigateway", "ready", r.Name, r.Environment, p.Name, res.Endpoint)
	return nil
}

// DestroyAPIGateway tears down an API gateway resource (tofu destroy) and marks
// it destroyed.
func (s *Service) DestroyAPIGateway(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("apigateway %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	gp, ok := prov.(providers.APIGatewayProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support API gateways", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveDeployCreds(ctx, p, targetAccountOf(r))

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("apigateway destroy started", "name", r.Name)

	if err := gp.DestroyAPIGateway(ctx, providers.APIGatewayRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: apiGatewaySpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("apigateway destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("apigateway destroy complete", "name", r.Name)
	s.emit("apigateway", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyAPIGatewayAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyAPIGatewayAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyAPIGateway(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_apigateway failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyAPIGateway(ctx, name, env); err != nil {
			s.log.Error("async apigateway destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteAPIGatewayRecord forgets a terminal API gateway resource's tracking row (no tofu).
func (s *Service) DeleteAPIGatewayRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("apigateway %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("apigateway %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing apigateway record %q: %w", name, err)
	}
	s.log.Info("apigateway record removed", "name", name)
	return nil
}

// ListAPIGateways returns all API gateway resources with provider name + parsed spec.
func (s *Service) ListAPIGateways(ctx context.Context) ([]APIGatewaySummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "apigateway")
	if err != nil {
		return nil, fmt.Errorf("listing apigateway resources: %w", err)
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
	out := make([]APIGatewaySummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, APIGatewaySummary{Resource: r, Provider: names[r.ProviderID], Spec: apiGatewaySpecOf(r)})
	}
	return out, nil
}

// APIGatewayStatus returns one API gateway resource by name + environment.
func (s *Service) APIGatewayStatus(ctx context.Context, name, env string) (*APIGatewaySummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("apigateway %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("apigateway %q (env %q) not found", name, env)
	}
	summary := &APIGatewaySummary{Resource: r, Spec: apiGatewaySpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
