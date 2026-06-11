package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// ProjectProvisioner: access-vending via AWS IAM Identity Center, wrapping
// modules/aws-sso-project (group + permission set + account assignment).

var _ providers.ProjectProvisioner = (*Provider)(nil)

func (p *Provider) writeProjectVars(req providers.ProjectRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildProjectVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling project vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-sso-*.tfvars.json")
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

// PreflightProject validates the var mapping + the aws-sso-project module offline.
func (p *Provider) PreflightProject(ctx context.Context, req providers.ProjectRequest) error {
	_, cleanup, err := p.writeProjectVars(req)
	if err != nil {
		return err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.ssoProjectModDir, p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionProject creates/updates the project (tofu apply) for the workspace.
// Re-running with a changed user_names list reconciles group membership (the
// day-2 "add/remove member" path).
func (p *Provider) ProvisionProject(ctx context.Context, req providers.ProjectRequest) (*providers.ProjectResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.ssoProjectModDir, p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeProjectVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-aws-sso-*.tfplan")
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
	raw := make(map[string]any, len(outs))
	for k, v := range outs {
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			raw[k] = val
		}
	}
	memberCount := 0
	if v, ok := raw["member_count"]; ok {
		if f, ok := v.(float64); ok {
			memberCount = int(f)
		}
	}
	return &providers.ProjectResult{
		GroupID:          dbOutString(outs, "group_id"),
		GroupName:        dbOutString(outs, "group_name"),
		PermissionSetARN: dbOutString(outs, "permission_set_arn"),
		AccountID:        dbOutString(outs, "account_id"),
		MemberCount:      memberCount,
		RawOutputs:       raw,
	}, nil
}

// DestroyProject tears down the project for the request's workspace.
func (p *Provider) DestroyProject(ctx context.Context, req providers.ProjectRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.ssoProjectModDir, p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeProjectVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildProjectVars maps a ProjectRequest onto the modules/aws-sso-project inputs.
func buildProjectVars(req providers.ProjectRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := req.Name
	if name == "" {
		name = "opord-" + req.Workspace
	}
	vars := map[string]any{
		"region":                      cfgString(cfg, "region"),
		"project_name":                name,
		"account_id":                  spec.AccountID,
		"permission_set_name":         spec.PermissionSetName,
		"existing_permission_set_arn": spec.ExistingPermissionSetARN,
		"sso_instance_arn":            spec.SSOInstanceARN,
		"identity_store_id":           spec.IdentityStoreID,
	}
	// Lists must be omitted when empty: a nil Go slice marshals to JSON null,
	// which overrides the module defaults and breaks toset(var...) for_each.
	if len(spec.UserNames) > 0 {
		vars["user_names"] = spec.UserNames
	}
	if len(spec.ManagedPolicyARNs) > 0 {
		vars["managed_policy_arns"] = spec.ManagedPolicyARNs
	}
	// Optional strings with meaningful module defaults: omit when empty so the
	// default (PT8H / opord-) applies instead of an invalid empty override.
	if spec.SessionDuration != "" {
		vars["session_duration"] = spec.SessionDuration
	}
	if spec.GroupPrefix != "" {
		vars["group_prefix"] = spec.GroupPrefix
	}
	return vars
}
