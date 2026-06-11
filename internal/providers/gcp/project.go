package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// ProjectProvisioner: grant an IAM role on a project to members via
// modules/gcp-iam-access (the GCP form of the provider-neutral "project /
// access" primitive - on AWS it's Identity Center, on Azure an Entra group +
// RBAC, on GCP a project IAM role binding). Day-2 add/remove member = update the
// member list + re-apply (idempotent).

var _ providers.ProjectProvisioner = (*Provider)(nil)

func (p *Provider) writeProjectVars(req providers.ProjectRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildProjectVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gcp project vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-access-*.tfvars.json")
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
	_, cleanup, err := p.writeProjectVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.accessModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionProject(ctx context.Context, req providers.ProjectRequest) (*providers.ProjectResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.accessModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
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

	planFile, err := os.CreateTemp("", "opord-gcp-access-*.tfplan")
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
	return &providers.ProjectResult{
		GroupName:        outString(outs, "group_name"),
		PermissionSetARN: outString(outs, "role"),
		AccountID:        outString(outs, "project_id"),
		MemberCount:      outInt(outs, "member_count"),
		RawOutputs:       rawMap(outs),
	}, nil
}

func (p *Provider) DestroyProject(ctx context.Context, req providers.ProjectRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.accessModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
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

// buildProjectVars maps a ProjectRequest onto the modules/gcp-iam-access inputs.
// The project defaults to the provider's project_id; spec.AccountID overrides it.
func buildProjectVars(req providers.ProjectRequest) map[string]any {
	spec := req.Spec
	project := spec.AccountID
	if project == "" {
		project = cfgString(req.Config, "project_id")
	}
	prefix := spec.GroupPrefix
	if prefix == "" {
		prefix = "opord-"
	}
	members := append([]string{}, spec.UserNames...)
	// Workforce Identity Federation: grant the role to Entra groups via a
	// principalSet. The pool comes from the spec or the provider config (set up once
	// by modules/gcp-workforce-pool). Groups are dropped if no pool is configured.
	if len(spec.EntraGroupIDs) > 0 {
		pool := spec.WorkforcePoolID
		if pool == "" {
			pool = cfgString(req.Config, "workforce_pool_id")
		}
		for _, g := range spec.EntraGroupIDs {
			if pool == "" || g == "" {
				continue
			}
			members = append(members, fmt.Sprintf(
				"principalSet://iam.googleapis.com/locations/global/workforcePools/%s/group/%s", pool, g))
		}
	}
	vars := map[string]any{
		"project_id": project,
		"role":       spec.RoleName,
		"label":      prefix + req.Name,
	}
	if len(members) > 0 {
		vars["members"] = members
	}
	return vars
}
