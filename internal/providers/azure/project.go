package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// ProjectProvisioner: access-vending project via modules/azure-access. The
// provider-neutral ProjectSpec ("a group bound to a role on an account, with
// members") maps onto an Entra security group + an Azure RBAC role assignment
// at a subscription / resource-group scope - the Azure analog of the AWS IAM
// Identity Center project. Day-2 add/remove member = re-apply with an updated
// UserNames list (the module owns the full membership, so it's idempotent).
//
// Needs the OPORD service-principal to hold (a) a directory role that can manage
// groups (e.g. "Groups Administrator") + read users, and (b) Owner or User
// Access Administrator on the target scope to create the role assignment.

var _ providers.ProjectProvisioner = (*Provider)(nil)

func (p *Provider) accessModuleDir() string {
	return p.cfg.ModulesDir + "/azure-access"
}

func (p *Provider) writeAccessVars(req providers.ProjectRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildAzureProjectVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling azure access vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-access-*.tfvars.json")
	if err != nil {
		return "", noop, err
	}
	remove := func() { _ = os.Remove(f.Name()) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		remove()
		return "", noop, err
	}
	if err := f.Close(); err != nil {
		remove()
		return "", noop, err
	}
	return f.Name(), remove, nil
}

func (p *Provider) PreflightProject(ctx context.Context, req providers.ProjectRequest) error {
	_, cleanup, err := p.writeAccessVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.accessModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionProject(ctx context.Context, req providers.ProjectRequest) (*providers.ProjectResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.accessModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeAccessVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-access-*.tfplan")
	if err != nil {
		return nil, err
	}
	planPath := planFile.Name()
	_ = planFile.Close()
	defer os.Remove(planPath)

	if _, _, err := r.Plan(ctx, varsFile, planPath); err != nil {
		return nil, err
	}
	if _, err := r.Apply(ctx, planPath); err != nil {
		return nil, err
	}
	outs, err := r.Output(ctx)
	if err != nil {
		return nil, err
	}
	// Map the access outputs onto the provider-neutral ProjectResult: the
	// subscription (the "account" the group is granted access to) goes into
	// AccountID; the role assignment id stands in for PermissionSetARN.
	sub := req.Spec.SubscriptionID
	if sub == "" {
		sub = cfgString(req.Config, "subscription_id")
	}
	return &providers.ProjectResult{
		GroupID:          azureOutString(outs, "group_id"),
		GroupName:        azureOutString(outs, "group_name"),
		PermissionSetARN: azureOutString(outs, "role_assignment_id"),
		AccountID:        sub,
		MemberCount:      len(req.Spec.UserNames),
		RawOutputs:       rawMap(outs),
	}, nil
}

func (p *Provider) DestroyProject(ctx context.Context, req providers.ProjectRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.accessModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeAccessVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildAzureProjectVars maps the provider-neutral ProjectSpec onto modules/azure-access.
func buildAzureProjectVars(req providers.ProjectRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := req.Name
	if name == "" {
		name = "opord-" + req.Workspace
	}

	// subscription_id: spec wins, else the provider's default subscription.
	sub := spec.SubscriptionID
	if sub == "" {
		sub = cfgString(cfg, "subscription_id")
	}

	role := spec.RoleName
	if role == "" {
		role = "Reader"
	}

	vars := map[string]any{
		"subscription_id": sub,
		"resource_group":  spec.ResourceGroup,
		"project_name":    name,
		"role_name":       role,
		"pim_eligible":    spec.PIMEligible,
	}
	// Lists must be omitted when empty: a nil Go slice marshals to JSON null,
	// which overrides the module default [] and breaks the data source.
	if len(spec.UserNames) > 0 {
		vars["user_principal_names"] = spec.UserNames
	}
	if spec.GroupPrefix != "" {
		vars["group_prefix"] = spec.GroupPrefix
	}
	return vars
}
