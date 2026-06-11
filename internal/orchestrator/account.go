package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	credspkg "github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/creds"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/google/uuid"
)

// CreateAccountInput is the request to provision a member AWS account.
type CreateAccountInput struct {
	Name        string
	Environment string
	Provider    string
	Spec        models.AccountSpec
	DryRun      bool
}

// CreateAccountResult reports the outcome (dry-run summary, or persisted resource).
type CreateAccountResult struct {
	DryRun   bool
	Summary  string
	Resource *db.Resource
}

// AccountSummary is an account resource enriched for list/detail views.
type AccountSummary struct {
	Resource db.Resource
	Provider string
	Spec     models.AccountSpec
}

func accountSpecOf(r db.Resource) models.AccountSpec {
	var s models.AccountSpec
	_ = json.Unmarshal(r.Spec, &s)
	return s
}

func accountResultOf(r db.Resource) providers.AccountResult {
	var ar providers.AccountResult
	if len(r.Observed) > 0 {
		_ = json.Unmarshal(r.Observed, &ar)
	}
	return ar
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func validateAccountSpec(spec models.AccountSpec) error {
	var errs []string
	if spec.CSAID == "" {
		errs = append(errs, "csa_id is required")
	}
	if spec.CloudName == "" {
		errs = append(errs, "cloud_name is required (prod/stage/dev)")
	}
	// AWS-only: email is required to create a new AWS account via Organizations.
	// Skipped when this is an Azure spec (AzureMode != "") or a GCP spec
	// (GCPMode != "") - those don't use a root email.
	if spec.AzureMode == "" && spec.GCPMode == "" {
		if spec.AccountID == "" && !spec.Skip.CreateAccount && !emailRe.MatchString(spec.Email) {
			errs = append(errs, "email must be a valid address (required to create a new account)")
		}
	}
	// GCP-specific (ADR-0011): GCPMode signals a GCP spec. "create" derives the
	// project id from csa_id-cloud_name (org/folder/billing come from the provider
	// config); "adopt" wraps an existing project.
	switch spec.GCPMode {
	case "":
		// not a GCP account spec
	case "create":
		// nothing extra - csa_id + cloud_name (already required) drive the project id
	case "adopt":
		if spec.GCPProjectID == "" {
			errs = append(errs, "gcp_mode=adopt requires gcp_project_id")
		}
	default:
		errs = append(errs, "gcp_mode must be empty, create, or adopt")
	}
	if spec.CreateVPC && spec.VPCCidr != "" {
		if !regexp.MustCompile(`/22$`).MatchString(spec.VPCCidr) {
			errs = append(errs, "vpc_cidr must be a /22")
		}
	}
	// Azure-specific (ADR-0009): mode adopt requires subscription_id; mode
	// create requires billing_scope_id. Empty AzureMode passes through -
	// the provider treats empty as "not an Azure spec".
	switch spec.AzureMode {
	case "":
		// not an Azure account spec - no Azure-side validation
	case "adopt":
		if spec.AzureSubscriptionID == "" {
			errs = append(errs, "azure_mode=adopt requires azure_subscription_id")
		}
	case "create":
		if spec.AzureBillingScopeID == "" {
			errs = append(errs, "azure_mode=create requires azure_billing_scope_id (MCA invoice section URI)")
		}
	default:
		errs = append(errs, "azure_mode must be empty, adopt, or create")
	}
	// Azure secure-VNet CIDR is /22 like AWS (provider expects 3 /24 subnets).
	if spec.AzureVNetCIDR != "" && !regexp.MustCompile(`/22$`).MatchString(spec.AzureVNetCIDR) {
		errs = append(errs, "azure_vnet_cidr must be a /22")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid account spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// CreateAccount validates an account spec and (unless DryRun) persists it and
// provisions it in the background. Requires a provider implementing AccountProvisioner.
func (s *Service) CreateAccount(ctx context.Context, in CreateAccountInput) (*CreateAccountResult, error) {
	if in.Name == "" || in.Provider == "" {
		return nil, fmt.Errorf("account name and provider are required")
	}
	if err := validateAccountSpec(in.Spec); err != nil {
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
	ap, ok := prov.(providers.AccountProvisioner)
	if !ok {
		return nil, fmt.Errorf("provider %q does not support account provisioning", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	// Account factory: cross-account hops need assumed_role creds (federation_token
	// can't chain AssumeRole) - WithFactoryCreds picks aws_factory_creds_path.
	creds, _ := s.creds.Resolve(credspkg.WithFactoryCreds(ctx), p)

	if err := ap.PreflightAccount(ctx, providers.AccountRequest{
		Workspace: in.Name, Name: in.Name, Spec: in.Spec, Credentials: creds, Config: cfg,
	}); err != nil {
		return nil, fmt.Errorf("account preflight failed: %w", err)
	}

	if in.DryRun {
		vpc := "no VPC"
		if in.Spec.CreateVPC {
			vpc = "secure VPC"
		}
		summary := fmt.Sprintf("spec valid; account opord-%s-%s (%s) on %s", in.Spec.CSAID, in.Spec.CloudName, vpc, in.Provider)
		s.log.Info("account preflight ok", "csa_id", in.Spec.CSAID, "provider", in.Provider)
		return &CreateAccountResult{DryRun: true, Summary: summary}, nil
	}

	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return nil, fmt.Errorf("marshaling account spec: %w", err)
	}
	r, err := s.q.CreateResource(ctx, db.CreateResourceParams{
		Name:          in.Name,
		Environment:   env,
		ProviderID:    p.ID,
		Kind:          "account",
		Spec:          specJSON,
		TofuWorkspace: uuid.NewString(),
		TenantID:      tenantForCreate(ctx),
	})
	if err != nil {
		return nil, fmt.Errorf("creating account resource: %w", err)
	}
	s.log.Info("account resource created", "name", r.Name, "csa_id", in.Spec.CSAID)
	s.emit("account", "created", r.Name, env, in.Provider, in.Spec.CSAID)
	s.startProvisionAccount(r.ID)
	return &CreateAccountResult{Resource: &r}, nil
}

func (s *Service) startProvisionAccount(resourceID uuid.UUID) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueProvisionAccount(context.Background(), resourceID); err != nil {
			s.log.Error("enqueue provision_account failed; running in-process", "id", resourceID, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		_ = s.ProvisionAccountByID(ctx, resourceID)
	}()
}

// ProvisionAccountByID loads an account resource + provider, allocates a CIDR
// (when a secure VPC is requested), runs the layers, and records the outcome.
func (s *Service) ProvisionAccountByID(ctx context.Context, resourceID uuid.UUID) error {
	r, err := s.q.GetResource(ctx, resourceID)
	if err != nil {
		return fmt.Errorf("loading account resource: %w", err)
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
	ap, ok := prov.(providers.AccountProvisioner)
	if !ok {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("provider %q does not support account provisioning", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	// Account factory: cross-account hops need assumed_role creds (federation_token
	// can't chain AssumeRole) - WithFactoryCreds picks aws_factory_creds_path.
	creds, _ := s.creds.Resolve(credspkg.WithFactoryCreds(ctx), p)
	spec := accountSpecOf(r)

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "provisioning"})
	s.log.Info("account provisioning started", "name", r.Name, "workspace", r.TofuWorkspace)

	// Allocate a /22 from the Vault pool when a secure VPC is requested and no
	// explicit CIDR was given. Idempotent per workspace owner (re-runs reuse).
	cidr := spec.VPCCidr
	if spec.CreateVPC && cidr == "" {
		if s.ipam == nil {
			s.markVMFailed(ctx, r.ID)
			err := fmt.Errorf("create_vpc requested but no IPAM configured (set VAULT_* or supply vpc_cidr)")
			s.emit("account", "failed", r.Name, r.Environment, p.Name, err.Error())
			return err
		}
		allocated, aerr := s.ipam.Allocate(ctx, r.TofuWorkspace)
		if aerr != nil {
			s.markVMFailed(ctx, r.ID)
			s.emit("account", "failed", r.Name, r.Environment, p.Name, aerr.Error())
			return fmt.Errorf("cidr allocation: %w", aerr)
		}
		cidr = allocated
	}

	res, err := ap.ProvisionAccount(ctx, providers.AccountRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: spec, Credentials: creds, Config: cfg, AllocatedCIDR: cidr,
	})
	if err != nil {
		s.log.Error("account provisioning failed", "name", r.Name, "err", err)
		s.markVMFailed(ctx, r.ID)
		s.emit("account", "failed", r.Name, r.Environment, p.Name, err.Error())
		return err
	}
	obs, _ := json.Marshal(res)
	_, _ = s.q.UpdateResourceObserved(ctx, db.UpdateResourceObservedParams{ID: r.ID, Observed: obs, Status: "ready"})
	s.log.Info("account provisioning complete", "name", r.Name, "account_id", res.AccountID)
	s.emit("account", "ready", r.Name, r.Environment, p.Name, res.AccountID)

	// Grant-at-creation: vend the requested team access on the freshly-created
	// account/project/subscription (best-effort - the account is already provisioned).
	s.grantTeamAccess(ctx, p, r.Environment, spec, res.AccountID)
	return nil
}

// grantTeamAccess vends the requested team access on a freshly-created factory
// account/project/subscription by composing the `project` access primitive, so a
// team gets access AS PART of account creation (no separate grant step or console
// self-grant). Best-effort: a grant failure is logged, not fatal - the account is
// already provisioned and the team can be granted manually. The grant itself
// provisions durably via River (CreateProject enqueues a provision_project job).
func (s *Service) grantTeamAccess(ctx context.Context, p db.Provider, env string, spec models.AccountSpec, accountID string) {
	if len(spec.GrantTeam) == 0 || accountID == "" {
		return
	}
	pspec := models.ProjectSpec{UserNames: spec.GrantTeam}
	switch models.ProviderType(p.Type) {
	case models.ProviderGCP:
		pspec.AccountID = accountID // the GCP project id
		pspec.RoleName = firstNonEmpty(spec.GrantRole, "roles/viewer")
	case models.ProviderAzure:
		pspec.SubscriptionID = accountID
		pspec.RoleName = firstNonEmpty(spec.GrantRole, "Reader")
	case models.ProviderAWS:
		pspec.AccountID = accountID
		pspec.PermissionSetName = truncate("team-"+spec.CSAID, 32)
		pspec.ManagedPolicyARNs = []string{firstNonEmpty(spec.GrantRole, "arn:aws:iam::aws:policy/ReadOnlyAccess")}
	default:
		return // on-prem providers don't vend cloud access
	}
	name := truncate("access-"+spec.CSAID, 60)
	if _, err := s.CreateProject(ctx, CreateProjectInput{Name: name, Environment: env, Provider: p.Name, Spec: pspec}); err != nil {
		s.log.Warn("grant-at-creation: team-access grant failed (account is provisioned; grant manually)",
			"account", accountID, "err", err)
		return
	}
	s.log.Info("grant-at-creation: vended team access", "account", accountID, "members", len(spec.GrantTeam), "role", pspec.RoleName)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// DestroyAccount tears down an account's layers and releases its CIDR. The AWS
// account itself is not closed (guarded decommission - see runbook).
func (s *Service) DestroyAccount(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("account %q (env %q) not found: %w", name, env, err)
	}
	p, err := s.q.GetProvider(ctx, r.ProviderID)
	if err != nil {
		return fmt.Errorf("provider lookup: %w", err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return err
	}
	ap, ok := prov.(providers.AccountProvisioner)
	if !ok {
		return fmt.Errorf("provider %q does not support account provisioning", p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	// Account factory: cross-account hops need assumed_role creds (federation_token
	// can't chain AssumeRole) - WithFactoryCreds picks aws_factory_creds_path.
	creds, _ := s.creds.Resolve(credspkg.WithFactoryCreds(ctx), p)

	// Recover the resolved account_id + allocated CIDR from the observed result.
	spec := accountSpecOf(r)
	observed := accountResultOf(r)
	if spec.AccountID == "" {
		spec.AccountID = observed.AccountID
	}

	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroying"})
	s.log.Info("account destroy started", "name", r.Name)

	if err := ap.DestroyAccount(ctx, providers.AccountRequest{
		Workspace: r.TofuWorkspace, Name: r.Name, Spec: spec, Credentials: creds, Config: cfg, AllocatedCIDR: observed.AllocatedCIDR,
	}); err != nil {
		s.markVMFailed(ctx, r.ID)
		return fmt.Errorf("account destroy failed: %w", err)
	}
	// Release the CIDR back to the pool (idempotent).
	if s.ipam != nil {
		if err := s.ipam.Release(ctx, r.TofuWorkspace); err != nil {
			s.log.Warn("cidr release failed (continuing)", "name", r.Name, "err", err)
		}
	}
	_, _ = s.q.UpdateResourceStatus(ctx, db.UpdateResourceStatusParams{ID: r.ID, Status: "destroyed"})
	s.log.Info("account destroy complete", "name", r.Name)
	s.emit("account", "destroyed", r.Name, r.Environment, p.Name, "")
	return nil
}

// DestroyAccountAsync enqueues a destroy job (or runs in-process without a queue).
func (s *Service) DestroyAccountAsync(name, env string) {
	if s.enqueuer != nil {
		if err := s.enqueuer.EnqueueDestroyAccount(context.Background(), name, env); err != nil {
			s.log.Error("enqueue destroy_account failed; running in-process", "name", name, "err", err)
		} else {
			return
		}
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		if err := s.DestroyAccount(ctx, name, env); err != nil {
			s.log.Error("async account destroy failed", "name", name, "env", env, "err", err)
		}
	}()
}

// DeleteAccountRecord forgets a terminal account's tracking row (no tofu, no
// account closure). Allowed only for destroyed/failed.
func (s *Service) DeleteAccountRecord(ctx context.Context, name, env string) error {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return fmt.Errorf("account %q (env %q) not found: %w", name, env, err)
	}
	switch r.Status {
	case "destroyed", "failed":
	default:
		return fmt.Errorf("account %q is %s - destroy it before removing the record", name, r.Status)
	}
	if err := s.q.DeleteResource(ctx, r.ID); err != nil {
		return fmt.Errorf("removing account record %q: %w", name, err)
	}
	s.log.Info("account record removed", "name", name)
	return nil
}

// ListAccounts returns all account resources with provider name + parsed spec.
func (s *Service) ListAccounts(ctx context.Context) ([]AccountSummary, error) {
	rs, err := s.q.ListResourcesByKind(ctx, "account")
	if err != nil {
		return nil, fmt.Errorf("listing accounts: %w", err)
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
	out := make([]AccountSummary, 0, len(rs))
	for _, r := range rs {
		if scoped && !resourceVisible(r, tid) {
			continue
		}
		out = append(out, AccountSummary{Resource: r, Provider: names[r.ProviderID], Spec: accountSpecOf(r)})
	}
	return out, nil
}

// AccountStatus returns one account resource by name + environment.
func (s *Service) AccountStatus(ctx context.Context, name, env string) (*AccountSummary, error) {
	if env == "" {
		env = "dev"
	}
	r, err := s.q.GetResourceByName(ctx, db.GetResourceByNameParams{Name: name, Environment: env})
	if err != nil {
		return nil, fmt.Errorf("account %q (env %q) not found: %w", name, env, err)
	}
	if tid, scoped := scopeTenant(ctx); scoped && !resourceVisible(r, tid) {
		return nil, fmt.Errorf("account %q (env %q) not found", name, env)
	}
	summary := &AccountSummary{Resource: r, Spec: accountSpecOf(r)}
	if p, err := s.q.GetProvider(ctx, r.ProviderID); err == nil {
		summary.Provider = p.Name
	}
	return summary, nil
}
