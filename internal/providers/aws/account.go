package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// AccountProvisioner: the OPORD-orchestrated account factory. ProvisionAccount
// sequences the layers - L1 create_account in the MASTER account (Organizations
// CreateAccount; tofu apply blocks until ACTIVE, handling the async + eventual
// consistency), then L2-L5 IN the member account via assume_role into the
// bootstrap role. Each layer is its own workspace-isolated tofu run.

var _ providers.AccountProvisioner = (*Provider)(nil)

func writeTfvars(prefix string, vars map[string]any) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(vars)
	if err != nil {
		return "", noop, err
	}
	f, err := os.CreateTemp("", prefix+"-*.tfvars.json")
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

// runLayerApply runs init to select workspace to plan to apply on one layer module
// and returns its outputs.
func (p *Provider) runLayerApply(ctx context.Context, moduleDir, workspace string, vars map[string]any, creds map[string]string, region string) (map[string]json.RawMessage, error) {
	r := tofu.New(p.cfg.TofuBin, moduleDir, p.log)
	r.SetEnv(awsTofuEnv(creds, nil, region))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := writeTfvars("opord-acct", vars)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	planFile, err := os.CreateTemp("", "opord-acct-*.tfplan")
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
	return r.Output(ctx)
}

func (p *Provider) runLayerDestroy(ctx context.Context, moduleDir, workspace string, vars map[string]any, creds map[string]string, region string) error {
	r := tofu.New(p.cfg.TofuBin, moduleDir, p.log)
	r.SetEnv(awsTofuEnv(creds, nil, region))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := writeTfvars("opord-acct", vars)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// tagsForAccount builds the standard tag set for account resources.
func tagsForAccount(spec models.AccountSpec) map[string]string {
	t := map[string]string{
		"csa_id":      spec.CSAID,
		"owner":       spec.Owner,
		"environment": spec.CloudName,
	}
	if spec.ProjectID != "" {
		t["project"] = spec.ProjectID
	}
	return t
}

// PreflightAccount validates the layer modules that would run, offline.
func (p *Provider) PreflightAccount(ctx context.Context, req providers.AccountRequest) error {
	dirs := []string{p.accountModDir, p.accountBaselineModDir, p.securityBaselineModDir}
	if req.Spec.CreateVPC {
		dirs = append(dirs, p.secureVpcModDir)
	}
	for _, d := range dirs {
		r := tofu.New(p.cfg.TofuBin, d, p.log)
		r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
		if err := r.InitBackendless(ctx); err != nil {
			return fmt.Errorf("preflight init %s: %w", d, err)
		}
		if err := r.Validate(ctx); err != nil {
			return fmt.Errorf("preflight validate %s: %w", d, err)
		}
	}
	return nil
}

// ProvisionAccount creates the member account (L1) and runs the enabled layers
// (L2 baseline to L3 access to L4 secure VPC to L5 security baseline).
func (p *Provider) ProvisionAccount(ctx context.Context, req providers.AccountRequest) (*providers.AccountResult, error) {
	spec := req.Spec
	region := cfgStringDefault(req.Config, "region", "us-east-1")
	vpcRegion := spec.VPCRegion
	if vpcRegion == "" {
		vpcRegion = region
	}
	namePrefix := "opord-" + spec.CSAID
	tags := tagsForAccount(spec)
	layers := map[string]string{}
	raw := map[string]any{}
	usedCIDR := ""

	// --- L1: create the account (master-level; no assume_role) ---
	accountID := spec.AccountID
	accessRoleArn := ""
	if accountID == "" && !spec.Skip.CreateAccount {
		accountName := fmt.Sprintf("opord-%s-%s", spec.CSAID, spec.CloudName)
		outs, err := p.runLayerApply(ctx, p.accountModDir, req.Workspace+"-account", map[string]any{
			"region":       region,
			"account_name": accountName,
			"email":        spec.Email,
			"ou_id":        cfgString(req.Config, "ou_id"),
			"tags":         tags,
		}, req.Credentials, region)
		if err != nil {
			return nil, fmt.Errorf("L1 create account: %w", err)
		}
		accountID = dbOutString(outs, "account_id")
		accessRoleArn = dbOutString(outs, "access_role_arn")
		layers["account"] = "ready"
	}
	if accountID == "" {
		return nil, fmt.Errorf("no account id (set account_id, or do not skip create_account)")
	}
	if accessRoleArn == "" {
		accessRoleArn = fmt.Sprintf("arn:aws:iam::%s:role/OrganizationAccountAccessRole", accountID)
	}
	raw["account_id"] = accountID

	// --- Setup: strip default VPCs in all regions (security hardening, the
	// reference's PHASE 4 / setup_aws_account). Runs in the member account via
	// assume_role; idempotent. ---
	if !spec.Skip.DeleteDefaultVPCs {
		if err := p.deleteDefaultVPCs(ctx, req.Credentials, region, accessRoleArn); err != nil {
			return nil, fmt.Errorf("delete default VPCs: %w", err)
		}
		layers["default_vpcs"] = "deleted"
	}

	// --- L2: baseline ---
	if !spec.Skip.Baseline {
		budget := spec.MonthlyBudgetUSD
		if budget == 0 {
			budget = 500
		}
		if _, err := p.runLayerApply(ctx, p.accountBaselineModDir, req.Workspace+"-baseline", map[string]any{
			"region":             region,
			"assume_role_arn":    accessRoleArn,
			"name":               namePrefix,
			"monthly_budget_usd": budget,
			"tags":               tags,
		}, req.Credentials, region); err != nil {
			return nil, fmt.Errorf("L2 baseline: %w", err)
		}
		layers["baseline"] = "ready"
	}

	// --- L3: access (SAML). Runs only when SAML metadata is supplied via config;
	// otherwise vend access through the `project` (Identity Center) primitive. ---
	if !spec.Skip.Access {
		if saml := cfgString(req.Config, "saml_metadata_document"); saml != "" {
			if _, err := p.runLayerApply(ctx, p.accountAccessModDir, req.Workspace+"-access", map[string]any{
				"region":                 region,
				"assume_role_arn":        accessRoleArn,
				"name":                   namePrefix,
				"saml_metadata_document": saml,
			}, req.Credentials, region); err != nil {
				return nil, fmt.Errorf("L3 access: %w", err)
			}
			layers["access"] = "ready"
		} else {
			layers["access"] = "skipped (no SAML metadata; use the project primitive for Identity Center)"
		}
	}

	// --- L4: secure VPC (optional) ---
	if spec.CreateVPC && !spec.Skip.SecureVPC {
		cidr := req.AllocatedCIDR
		if cidr == "" {
			cidr = spec.VPCCidr
		}
		if cidr == "" {
			return nil, fmt.Errorf("L4 secure vpc: no CIDR (IPAM allocation or vpc_cidr required)")
		}
		if _, err := p.runLayerApply(ctx, p.secureVpcModDir, req.Workspace+"-network", map[string]any{
			"region":          vpcRegion,
			"assume_role_arn": accessRoleArn,
			"name":            namePrefix,
			"vpc_cidr":        cidr,
			"enable_nat":      spec.EnableNAT,
			"tags":            tags,
		}, req.Credentials, vpcRegion); err != nil {
			return nil, fmt.Errorf("L4 secure vpc: %w", err)
		}
		usedCIDR = cidr
		layers["network"] = "ready (" + cidr + ")"
	}

	// --- L5: security baseline ---
	if !spec.Skip.SecurityBaseline {
		if _, err := p.runLayerApply(ctx, p.securityBaselineModDir, req.Workspace+"-security", map[string]any{
			"region":          region,
			"assume_role_arn": accessRoleArn,
			"name":            namePrefix,
			"tags":            tags,
		}, req.Credentials, region); err != nil {
			return nil, fmt.Errorf("L5 security baseline: %w", err)
		}
		layers["security"] = "ready"
	}

	return &providers.AccountResult{
		AccountID:     accountID,
		AccessRoleARN: accessRoleArn,
		AllocatedCIDR: usedCIDR,
		Layers:        layers,
		RawOutputs:    raw,
	}, nil
}

// DestroyAccount tears down the member-account layers in reverse (L5 to L2). The
// account itself is NOT closed here - closure is a guarded decommission action
// (90-day, irreversible). See runbook 02-decommissioning.
func (p *Provider) DestroyAccount(ctx context.Context, req providers.AccountRequest) error {
	spec := req.Spec
	region := cfgStringDefault(req.Config, "region", "us-east-1")
	vpcRegion := spec.VPCRegion
	if vpcRegion == "" {
		vpcRegion = region
	}
	accountID := spec.AccountID
	if accountID == "" {
		return fmt.Errorf("destroy account: account_id required")
	}
	accessRoleArn := fmt.Sprintf("arn:aws:iam::%s:role/OrganizationAccountAccessRole", accountID)
	namePrefix := "opord-" + spec.CSAID

	if !spec.Skip.SecurityBaseline {
		if err := p.runLayerDestroy(ctx, p.securityBaselineModDir, req.Workspace+"-security", map[string]any{
			"region": region, "assume_role_arn": accessRoleArn, "name": namePrefix,
		}, req.Credentials, region); err != nil {
			return fmt.Errorf("destroy L5 security: %w", err)
		}
	}
	if spec.CreateVPC && !spec.Skip.SecureVPC {
		cidr := req.AllocatedCIDR
		if cidr == "" {
			cidr = spec.VPCCidr
		}
		if cidr != "" {
			if err := p.runLayerDestroy(ctx, p.secureVpcModDir, req.Workspace+"-network", map[string]any{
				"region": vpcRegion, "assume_role_arn": accessRoleArn, "name": namePrefix, "vpc_cidr": cidr, "enable_nat": spec.EnableNAT,
			}, req.Credentials, vpcRegion); err != nil {
				return fmt.Errorf("destroy L4 network: %w", err)
			}
		}
	}
	if !spec.Skip.Baseline {
		if err := p.runLayerDestroy(ctx, p.accountBaselineModDir, req.Workspace+"-baseline", map[string]any{
			"region": region, "assume_role_arn": accessRoleArn, "name": namePrefix,
		}, req.Credentials, region); err != nil {
			return fmt.Errorf("destroy L2 baseline: %w", err)
		}
	}
	return nil
}
