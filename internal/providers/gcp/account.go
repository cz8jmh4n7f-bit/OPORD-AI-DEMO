package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// AccountProvisioner: the GCP project factory (ADR-0011) - a managed
// project provisioned as layered, workspace-isolated tofu modules, mirroring the
// AWS account factory and the Azure subscription factory. Layers (each its own
// pg-backend workspace so re-runs + partial destroys are clean):
//
//	L1 project   - gcp-account-project   (folder + project, auto_create_network=false)
//	L2 apis      - gcp-account-apis       (service enablement + propagation wait)
//	L3 security  - gcp-account-security   (KMS/CMEK + log sink + metrics)
//	L4 vpc       - gcp-account-vpc        (secure VPC + 3 /24 + ZTNA firewall)   [if create_vpc]
//	L5 orgpolicy - gcp-account-org-policy (project org-policy constraints)
//	L6 iam       - gcp-account-iam        (custom role + bindings, existing members)
//
// The organization folder + billing account come from the PROVIDER CONFIG
// (OpenBao): gcp_folder_parent ("organizations/NNN" | "folders/NNN") +
// gcp_billing_account. v1 = create mode + existing-member IAM. Destroy runs the
// layers in reverse; the project is removed per its deletion_policy (DELETE for
// dev - 30-day id reservation - / PREVENT for prod).

var _ providers.AccountProvisioner = (*Provider)(nil)

func (p *Provider) accountModuleDir(layer string) string {
	return filepath.Join(p.cfg.ModulesDir, "gcp-account-"+layer)
}

func writeAccountVars(vars map[string]any) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(vars)
	if err != nil {
		return "", noop, fmt.Errorf("marshaling account vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-acct-*.tfvars.json")
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

func (p *Provider) runAccountLayerApply(ctx context.Context, layer, workspace string, vars, creds, cfg map[string]any, region string) (map[string]json.RawMessage, error) {
	r := tofu.New(p.cfg.TofuBin, p.accountModuleDir(layer), p.log)
	r.SetEnv(gcpTofuEnv(toStrMap(creds), cfg, region))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := writeAccountVars(vars)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	planFile, err := os.CreateTemp("", "opord-gcp-acct-*.tfplan")
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

func (p *Provider) runAccountLayerDestroy(ctx context.Context, layer, workspace string, vars, creds, cfg map[string]any, region string) error {
	r := tofu.New(p.cfg.TofuBin, p.accountModuleDir(layer), p.log)
	r.SetEnv(gcpTofuEnv(toStrMap(creds), cfg, region))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := writeAccountVars(vars)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// toStrMap narrows a map[string]any of string creds to map[string]string.
func toStrMap(m map[string]any) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

// PreflightAccount validates the layer modules that would run, offline.
func (p *Provider) PreflightAccount(ctx context.Context, req providers.AccountRequest) error {
	layers := []string{"project", "apis", "security", "org-policy", "iam"}
	if req.Spec.CreateVPC {
		layers = append(layers, "vpc")
	}
	for _, l := range layers {
		r := tofu.New(p.cfg.TofuBin, p.accountModuleDir(l), p.log)
		r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
		if err := r.InitBackendless(ctx); err != nil {
			return fmt.Errorf("preflight init gcp-account-%s: %w", l, err)
		}
		if err := r.Validate(ctx); err != nil {
			return fmt.Errorf("preflight validate gcp-account-%s: %w", l, err)
		}
	}
	return nil
}

// ProvisionAccount creates the project (L1) and runs the enabled layers.
func (p *Provider) ProvisionAccount(ctx context.Context, req providers.AccountRequest) (*providers.AccountResult, error) {
	spec := req.Spec
	cfg := req.Config
	creds := anyMap(req.Credentials)
	region := spec.VPCRegion
	if region == "" {
		region = cfgStringDefault(cfg, "region", "europe-west1")
	}
	folderParent := firstNonEmpty(cfgString(cfg, "gcp_folder_parent"), cfgString(cfg, "gcp_folder_id"))
	billing := cfgString(cfg, "gcp_billing_account")

	layers := map[string]string{}
	raw := map[string]any{}

	// --- L1: project (folder + project + billing) ---
	projectID := spec.GCPProjectID
	projectNumber := ""
	if spec.GCPMode != "adopt" {
		outs, err := p.runAccountLayerApply(ctx, "project", req.Workspace+"-project", map[string]any{
			"csa_id":          spec.CSAID,
			"cloud_name":      spec.CloudName,
			"folder_parent":   folderParent,
			"billing_account": billing,
			"owner":           spec.Owner,
			"managed_by":      "opord",
			"cost_center":     spec.CSAID,
		}, creds, cfg, region)
		if err != nil {
			return nil, fmt.Errorf("L1 project: %w", err)
		}
		projectID = outString(outs, "project_id")
		projectNumber = outString(outs, "project_number")
		layers["project"] = "ready"
		raw["project"] = rawMap(outs)
	} else {
		layers["project"] = "adopted"
	}
	if projectID == "" {
		return nil, fmt.Errorf("no project id (L1 output empty / adopt mode without gcp_project_id)")
	}

	// --- L2: apis ---
	if !spec.Skip.GCPApis {
		outs, err := p.runAccountLayerApply(ctx, "apis", req.Workspace+"-apis", map[string]any{
			"project_id":        projectID,
			"create_vpc":        spec.CreateVPC,
			"enable_org_policy": !spec.Skip.GCPOrgPolicy,
		}, creds, cfg, region)
		if err != nil {
			return nil, fmt.Errorf("L2 apis: %w", err)
		}
		layers["apis"] = "ready"
		raw["apis"] = rawMap(outs)
	}

	// --- L3: security ---
	if !spec.Skip.GCPSecurity {
		outs, err := p.runAccountLayerApply(ctx, "security", req.Workspace+"-security", map[string]any{
			"project_id":     projectID,
			"project_number": projectNumber,
			"csa_id":         spec.CSAID,
		}, creds, cfg, region)
		if err != nil {
			return nil, fmt.Errorf("L3 security: %w", err)
		}
		layers["security"] = "ready"
		raw["security"] = rawMap(outs)
	}

	// --- L4: secure VPC (CIDR from IPAM via req.AllocatedCIDR) ---
	usedCIDR := ""
	if spec.CreateVPC && !spec.Skip.GCPSecureVPC {
		cidr := firstNonEmpty(req.AllocatedCIDR, spec.VPCCidr)
		if cidr == "" {
			return nil, fmt.Errorf("L4 vpc: no CIDR (IPAM allocation or vpc_cidr required)")
		}
		outs, err := p.runAccountLayerApply(ctx, "vpc", req.Workspace+"-vpc", map[string]any{
			"project_id":          projectID,
			"csa_id":              spec.CSAID,
			"region":              region,
			"vpc_cidr":            cidr,
			"allow_inbound_cidrs": cfgListDefault(spec.GCPAllowInboundCIDRs, []string{"0.0.0.0/0"}),
		}, creds, cfg, region)
		if err != nil {
			return nil, fmt.Errorf("L4 vpc: %w", err)
		}
		usedCIDR = cidr
		layers["vpc"] = "ready"
		raw["vpc"] = rawMap(outs)
	}

	// --- L5: org policy ---
	if !spec.Skip.GCPOrgPolicy {
		vars := map[string]any{"project_id": projectID}
		if len(spec.GCPAllowedLocations) > 0 {
			vars["allowed_locations"] = spec.GCPAllowedLocations
		}
		if doms := cfgStringListDefault(cfg, "gcp_allowed_member_domains", nil); len(doms) > 0 {
			vars["allowed_member_domains"] = doms
		}
		outs, err := p.runAccountLayerApply(ctx, "org-policy", req.Workspace+"-orgpolicy", vars, creds, cfg, region)
		if err != nil {
			return nil, fmt.Errorf("L5 org-policy: %w", err)
		}
		layers["orgpolicy"] = "ready"
		raw["orgpolicy"] = rawMap(outs)
	}

	// --- L6: iam (existing members) ---
	if !spec.Skip.GCPIAM {
		vars := map[string]any{"project_id": projectID, "csa_id": spec.CSAID, "project_number": projectNumber}
		if b := gcpAccountBindings(spec, cfg); len(b) > 0 {
			vars["bindings"] = b
		}
		outs, err := p.runAccountLayerApply(ctx, "iam", req.Workspace+"-iam", vars, creds, cfg, region)
		if err != nil {
			return nil, fmt.Errorf("L6 iam: %w", err)
		}
		layers["iam"] = "ready"
		raw["iam"] = rawMap(outs)
	}

	return &providers.AccountResult{
		AccountID:     projectID,
		AllocatedCIDR: usedCIDR,
		Layers:        layers,
		RawOutputs:    raw,
	}, nil
}

// DestroyAccount tears down the account. CREATE mode deletes ONLY the project
// (target-destroy google_project.this): that removes ALL child resources -
// including the KMS keyring GCP refuses to delete individually - and avoids a full
// `tofu destroy` racing the per-CSA folder (a folder can't be deleted while its
// project is pending-deletion). The L2-L6 workspaces' stale state and the per-CSA
// folder (free) are left: workspaces are keyed by the account's unique id and
// never reused, and the folder becomes deletable once the project finishes its
// 30-day purge. ADOPT mode (the project persists) tears the layers down in
// reverse, best-effort (the KMS keyring lingers but is free, in the user's project).
func (p *Provider) DestroyAccount(ctx context.Context, req providers.AccountRequest) error {
	spec := req.Spec
	cfg := req.Config
	creds := anyMap(req.Credentials)
	region := spec.VPCRegion
	if region == "" {
		region = cfgStringDefault(cfg, "region", "europe-west1")
	}

	if spec.GCPMode != "adopt" {
		folderParent := firstNonEmpty(cfgString(cfg, "gcp_folder_parent"), cfgString(cfg, "gcp_folder_id"))
		billing := cfgString(cfg, "gcp_billing_account")
		return p.runAccountLayerDestroyTarget(ctx, "project", req.Workspace+"-project", map[string]any{
			"csa_id": spec.CSAID, "cloud_name": spec.CloudName, "folder_parent": folderParent, "billing_account": billing,
		}, creds, cfg, region, "google_project.this")
	}

	cidr := firstNonEmpty(req.AllocatedCIDR, spec.VPCCidr)
	projectID := spec.GCPProjectID
	var firstErr error
	note := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if !spec.Skip.GCPIAM {
		note(p.runAccountLayerDestroy(ctx, "iam", req.Workspace+"-iam", map[string]any{"project_id": projectID, "csa_id": spec.CSAID}, creds, cfg, region))
	}
	if !spec.Skip.GCPOrgPolicy {
		note(p.runAccountLayerDestroy(ctx, "org-policy", req.Workspace+"-orgpolicy", map[string]any{"project_id": projectID}, creds, cfg, region))
	}
	if spec.CreateVPC && !spec.Skip.GCPSecureVPC && cidr != "" {
		note(p.runAccountLayerDestroy(ctx, "vpc", req.Workspace+"-vpc", map[string]any{
			"project_id": projectID, "csa_id": spec.CSAID, "region": region, "vpc_cidr": cidr,
		}, creds, cfg, region))
	}
	if !spec.Skip.GCPSecurity {
		note(p.runAccountLayerDestroy(ctx, "security", req.Workspace+"-security", map[string]any{"project_id": projectID, "csa_id": spec.CSAID}, creds, cfg, region))
	}
	if !spec.Skip.GCPApis {
		note(p.runAccountLayerDestroy(ctx, "apis", req.Workspace+"-apis", map[string]any{"project_id": projectID}, creds, cfg, region))
	}
	return firstErr
}

// runAccountLayerDestroyTarget destroys only `target` within a layer workspace
// (see tofu.Runner.DestroyTarget) - used for the project-only create-mode teardown.
func (p *Provider) runAccountLayerDestroyTarget(ctx context.Context, layer, workspace string, vars, creds, cfg map[string]any, region, target string) error {
	r := tofu.New(p.cfg.TofuBin, p.accountModuleDir(layer), p.log)
	r.SetEnv(gcpTofuEnv(toStrMap(creds), cfg, region))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := writeAccountVars(vars)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.DestroyTarget(ctx, varsFile, target)
}

// gcpAccountBindings maps the spec owner + role onto least-privilege predefined
// GCP roles (never roles/owner|roles/editor for humans). The owner must resolve
// to an email - if it has no domain, the provider config's gcp_domain is appended;
// without one the binding is skipped (the project + custom role still apply).
func gcpAccountBindings(spec models.AccountSpec, cfg map[string]any) []map[string]string {
	member := ownerMember(spec.Owner, cfgString(cfg, "gcp_domain"))
	if member == "" {
		return nil
	}
	out := make([]map[string]string, 0, 3)
	for _, role := range gcpAccountRoles(spec.Role) {
		out = append(out, map[string]string{"member": member, "role": role})
	}
	return out
}

// gcpAccountRoles maps a logical role (Admin/Manager/ReadOnly/Custom1) onto
// non-primitive predefined GCP roles.
func gcpAccountRoles(role string) []string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return []string{"roles/compute.admin", "roles/iam.securityAdmin", "roles/resourcemanager.projectIamAdmin"}
	case "manager":
		return []string{"roles/viewer", "roles/compute.viewer"}
	case "readonly", "":
		return []string{"roles/viewer"}
	default:
		// treat an explicit predefined role string as-is
		if strings.HasPrefix(role, "roles/") {
			return []string{role}
		}
		return []string{"roles/viewer"}
	}
}

// ownerMember turns an owner (email or bare username) into an IAM member string.
func ownerMember(owner, domain string) string {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return ""
	}
	if strings.Contains(owner, "@") {
		return "user:" + owner
	}
	if domain != "" {
		return "user:" + owner + "@" + domain
	}
	return "" // can't form a valid member without a domain
}

func anyMap(m map[string]string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func cfgListDefault(v, def []string) []string {
	if len(v) > 0 {
		return v
	}
	return def
}
