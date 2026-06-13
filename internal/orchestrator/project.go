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

// CreateProjectInput is the request to provision an access-vending project.
type CreateProjectInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.ProjectSpec
	DryRun      bool
}

// CreateProjectResult reports the outcome (dry-run summary, or persisted resource).
type CreateProjectResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// ProjectSummary is a project resource enriched for list/detail views.
type ProjectSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.ProjectSpec
}

func projectSpecOf(r db.Resource) models.ProjectSpec {
	var s models.ProjectSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

var accountIDRe = regexp.MustCompile(`^[0-9]{12}$`)

// validateProjectSpec is provider-aware: AWS (IAM Identity Center) needs a
// 12-digit account + a permission set; Azure (Entra group + Azure RBAC) needs a
// subscription + a role. The provider type isn't known until the provider is
// looked up, so this runs after that lookup.
func validateProjectSpec(spec models.ProjectSpec, providerType string) error {
	var errs []string
	if providerType == string(models.ProviderAzure) {
		// subscription_id is optional - it defaults to the provider's configured
		// subscription. role_name is the one required input.
		if spec.RoleName == "" {
			errs = append(errs, "role_name is required (e.g. Reader, Contributor)")
		}
	} else if providerType == string(models.ProviderGCP) {
		// GCP grants an IAM role to members on a project; role_name is required
		// (e.g. roles/viewer). The project defaults to the provider's project_id
		// (account_id can override it).
		if spec.RoleName == "" {
			errs = append(errs, "role_name is required (e.g. roles/viewer, roles/editor)")
		}
	} else {
		if !accountIDRe.MatchString(spec.AccountID) {
			errs = append(errs, "account_id must be a 12-digit AWS account ID")
		}
		if spec.PermissionSetName == "" && spec.ExistingPermissionSetARN == "" {
			errs = append(errs, "set permission_set_name (to create one) or existing_permission_set_arn (to reference one)")
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid project spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateProject validates a project spec and (unless DryRun) persists it and
// provisions it in the background. Requires a provider implementing ProjectProvisioner.
func (s *Service) CreateProject(ctx context.Context, in CreateProjectInput) (*CreateProjectResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("project name and provider are required")
	}
	env := in.Environment
	if env == "" {
		env = "dev"
	}

	p, err := s.q.GetProviderByName(ctx, in.Provider)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found (register it with `opord provider add`): %w", in.Provider, err)
	}
	if err := validateProjectSpec(in.Spec, p.Type); err != nil {
		return nil, err
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return nil, err
	}
	pp, ok := prov.(providers.ProjectProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support access-vending projects", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)

	if err := pp.PreflightProject(ctx, providers.ProjectRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("project preflight failed: %w", err)
	}

	if in.DryRun {
		summary := fmt.Sprintf("spec valid; project %q to account %s (%d user(s)) on %s", in.Name, in.Spec.AccountID, len(in.Spec.UserNames), in.Provider)
		s.log.Info("project preflight ok", "name", in.Name, "provider", in.Provider)
		return &CreateProjectResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling project spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "project",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating project resource: %w", err)
	}
	s.log.Info("project resource created", "name", r.Name, "account", in.Spec.AccountID)
	s.emit("project", "created", r.Name, env, in.Provider, in.Spec.AccountID)
	s.startProvisionProject(r.ID)
	return &CreateProjectResult{Resource: &r}, nil
}

func (s *Service) startProvisionProject(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionProject(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_project failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionProjectByID(ctx, resourceID)
	}()
}

// ProvisionProjectByID loads a project resource + its provider and runs the tofu
// apply, recording the outcome. Status flows provisioning -> ready/failed.
func (s *Service) ProvisionProjectByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading project resource: %w", err)
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
	pp, ok := prov.(providers.ProjectProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support access-vending projects", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveProjectCreds(ctx, p)

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("project provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	res, err := pp.ProvisionProject(ctx, providers.ProjectRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: projectSpecOf(r), Credentials: creds, Config: cfg,
	})
	if err != nil {
		s.log.Error("project provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID, err)
		s.emit("project", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("project provisioning complete", "name", r.Name, "group", res.GroupName)
	s.emit("project", "ready", r.Name, r.Environment, p.Name, res.GroupName)
	return nil
}

// SetProjectMembers replaces the project's member list and re-provisions (tofu
// apply reconciles group membership - idempotent). This is the day-2 "add/remove
// a user" path.
func (s *Service) SetProjectMembers(ctx context.Context, name, env string, members []string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("project %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return fmt.Errorf("project %q (env %q) not found", name, env)
	}
	spec := projectSpecOf(r)
	spec.UserNames = dedupeStrings(members)
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshaling project spec: %w", err)
	}
	if _, err := s.q.UpdateResourceSpec(ctx, db.UpdateResourceSpecParams{ID: r.ID, Spec: specJSON, Status: "provisioning"}); err != nil {
		return fmt.Errorf("updating project spec: %w", err)
	}
	s.log.Info("project members updated", "name", r.Name, "members", len(spec.UserNames))
	s.emit("project", "updating", r.Name, r.Environment, "", fmt.Sprintf("members to %d", len(spec.UserNames)))
	s.startProvisionProject(r.ID)
	return nil
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// DestroyProject tears down a project resource (tofu destroy) and marks it destroyed.
func (s *Service) DestroyProject(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("project %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	pp, ok := prov.(providers.ProjectProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support access-vending projects", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	creds, _ := s.resolveProjectCreds(ctx, p)

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("project destroy started", "name", r.Name)

	if err := pp.DestroyProject(ctx, providers.ProjectRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: projectSpecOf(r), Credentials: creds, Config: cfg,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("project destroy failed: %w", err)
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("project destroy complete", "name", r.Name)
	s.emit("project", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyProjectAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyProjectAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyProject(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_project failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyProject(ctx, name, env); err != nil {
			s.log.Error("async project destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteProjectRecord forgets a terminal project's tracking row (no tofu).
// Allowed only for destroyed/failed - destroy a live project first.
func (s *Service) DeleteProjectRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("project %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("project %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing project record %q: %w", name, err)
	}
	s.log.Info("project record removed", "name", name)
	return nil
}

// ListProjects returns all project resources with provider name + parsed spec.
func (s *Service) ListProjects(ctx context.Context) ([]ProjectSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "project")
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
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
	out := make([]ProjectSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, ProjectSummary{Resource: r, Provider: names[r.ProviderID], Spec: projectSpecOf(r)})
	}
	return out, nil
}

// ProjectStatus returns one project resource by name + environment.
func (s *Service) ProjectStatus(ctx context.Context, name, env string) (*ProjectSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("project %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("project %q (env %q) not found", name, env)
	}
	summary := &ProjectSummary{Resource: r, Spec: projectSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
