package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// AccountProvisioner: the Azure subscription factory (ADR-0009). Mirrors the
// AWS account factory shape - one provider call sequences L1 (subscription
// adopt/create) to L2 (baseline) to KV (vault + CMK) to L5 (LAW + Activity Log)
// to L4 (secure VNet) to L3 (RBAC + Entra groups) to Policy. Each layer is a
// workspace-isolated tofu run with deterministic names so re-runs and partial
// destroys are clean.
//
// Why the L5-before-L4 ordering: L4's Flow Logs need a Storage Account, which
// L5 creates. Layer numbers reflect the *conceptual* order; the *runtime*
// dependency graph is what we follow here.

var _ providers.AccountProvisioner = (*Provider)(nil)

func writeAzureTfvars(prefix string, vars map[string]any) (string, func(), error) {
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

// runAccountLayerApply runs init to select workspace to plan to apply on one
// layer module and returns its outputs.
func (p *Provider) runAccountLayerApply(ctx context.Context, moduleDir, workspace string, vars map[string]any, creds map[string]string, cfg map[string]any, location string) (map[string]json.RawMessage, error) {
	r := tofu.New(p.cfg.TofuBin, moduleDir, p.log)
	r.SetEnv(azureTofuEnv(creds, cfg, location))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := writeAzureTfvars("opord-azacct", vars)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	planFile, err := os.CreateTemp("", "opord-azacct-*.tfplan")
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

func (p *Provider) runAccountLayerDestroy(ctx context.Context, moduleDir, workspace string, vars map[string]any, creds map[string]string, cfg map[string]any, location string) error {
	r := tofu.New(p.cfg.TofuBin, moduleDir, p.log)
	r.SetEnv(azureTofuEnv(creds, cfg, location))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := writeAzureTfvars("opord-azacct", vars)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

func tagsForAzureAccount(spec models.AccountSpec) map[string]string {
	return map[string]string{
		"Project":   "opord",
		"CsaId":     spec.CSAID,
		"Cloud":     spec.CloudName,
		"Owner":     spec.Owner,
		"ManagedBy": "opord",
	}
}

// PreflightAccount validates every layer module offline (no Azure API calls).
func (p *Provider) PreflightAccount(ctx context.Context, req providers.AccountRequest) error {
	dirs := []string{
		p.subscriptionModDir,
		p.subscriptionBaselineModDir,
		p.keyVaultBaselineModDir,
		p.subscriptionRBACModDir,
		p.secureVNetModDir,
		p.securityHardeningModDir,
		p.policyModDir,
	}
	for _, d := range dirs {
		r := tofu.New(p.cfg.TofuBin, d, p.log)
		r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
		if err := r.InitBackendless(ctx); err != nil {
			return fmt.Errorf("preflight %s: init: %w", d, err)
		}
		if err := r.Validate(ctx); err != nil {
			return fmt.Errorf("preflight %s: validate: %w", d, err)
		}
	}
	return nil
}

// ProvisionAccount chains the layers. On any layer's failure the caller
// (orchestrator) decides whether to destroy the partial state - same contract
// as the AWS path.
func (p *Provider) ProvisionAccount(ctx context.Context, req providers.AccountRequest) (*providers.AccountResult, error) {
	spec := req.Spec
	tags := tagsForAzureAccount(spec)
	location := spec.AzureLocation
	if location == "" {
		location = cfgString(req.Config, "location")
	}
	if location == "" {
		location = "westeurope"
	}
	mode := spec.AzureMode
	if mode == "" {
		mode = "adopt"
	}

	// -------- Layer 1: subscription (adopt or create) --------
	l1Vars := map[string]any{
		"mode":              mode,
		"subscription_id":   spec.AzureSubscriptionID,
		"subscription_name": fmt.Sprintf("opord-%s-%s", spec.CSAID, spec.CloudName),
		"billing_scope_id":  spec.AzureBillingScopeID,
		"name_prefix":       "opord",
		"csa_id":            spec.CSAID,
		"csa_cloud_name":    spec.CloudName,
		"tags":              tags,
	}
	l1WS := workspaceName(req.Workspace, "l1")
	l1Outs, err := p.runAccountLayerApply(ctx, p.subscriptionModDir, l1WS, l1Vars, req.Credentials, req.Config, location)
	if err != nil {
		return nil, fmt.Errorf("L1 subscription: %w", err)
	}
	subscriptionID := outString(l1Outs, "subscription_id")
	subscriptionResourceID := outString(l1Outs, "subscription_resource_id")
	result := &providers.AccountResult{
		AccountID: subscriptionID,
		Layers: map[string]string{
			"subscription": "ready",
		},
	}

	// -------- Layer 2: baseline --------
	if !spec.Skip.Baseline {
		l2Vars := map[string]any{
			"subscription_id": subscriptionID,
			"location":        location,
			"name_prefix":     "opord",
			"csa_id":          spec.CSAID,
			"csa_cloud_name":  spec.CloudName,
			"tags":            tags,
		}
		if len(spec.AzureDefenderPlansStandard) > 0 {
			l2Vars["defender_plans_standard"] = spec.AzureDefenderPlansStandard
		}
		l2Outs, err := p.runAccountLayerApply(ctx, p.subscriptionBaselineModDir, workspaceName(req.Workspace, "l2"), l2Vars, req.Credentials, req.Config, location)
		if err != nil {
			return result, fmt.Errorf("L2 baseline: %w", err)
		}
		result.Layers["baseline"] = "ready"
		networkRG := outString(l2Outs, "network_rg_name")
		securityRG := outString(l2Outs, "security_rg_name")
		logsRG := outString(l2Outs, "logs_rg_name")

		// -------- Companion: Key Vault baseline --------
		var cmkVersionlessID string
		if !spec.Skip.AzureKeyVault {
			kvVars := map[string]any{
				"subscription_id":  subscriptionID,
				"security_rg_name": securityRG,
				"location":         location,
				"name_prefix":      "opord",
				"csa_id":           spec.CSAID,
				"tags":             tags,
			}
			kvOuts, err := p.runAccountLayerApply(ctx, p.keyVaultBaselineModDir, workspaceName(req.Workspace, "kv"), kvVars, req.Credentials, req.Config, location)
			if err != nil {
				return result, fmt.Errorf("KV baseline: %w", err)
			}
			result.Layers["key_vault"] = "ready"
			cmkVersionlessID = outString(kvOuts, "cmk_versionless_id")
		}

		// -------- Layer 5: security hardening (built before L4 so flow logs have a SA) --------
		var archiveStorageID string
		if !spec.Skip.AzureSecurityHardening {
			l5Vars := map[string]any{
				"subscription_id":          subscriptionID,
				"subscription_resource_id": subscriptionResourceID,
				"logs_rg_name":             logsRG,
				"location":                 location,
				"name_prefix":              "opord",
				"csa_id":                   spec.CSAID,
				"cmk_versionless_id":       cmkVersionlessID,
				"tags":                     tags,
			}
			l5Outs, err := p.runAccountLayerApply(ctx, p.securityHardeningModDir, workspaceName(req.Workspace, "l5"), l5Vars, req.Credentials, req.Config, location)
			if err != nil {
				return result, fmt.Errorf("L5 security hardening: %w", err)
			}
			result.Layers["security_hardening"] = "ready"
			archiveStorageID = outString(l5Outs, "archive_storage_account_id")
		}

		// -------- Layer 4: secure VNet --------
		if !spec.Skip.AzureSecureVNet && archiveStorageID != "" {
			vnetCIDR := spec.AzureVNetCIDR
			if vnetCIDR == "" {
				return result, fmt.Errorf("L4 secure VNet: AzureVNetCIDR is empty; the orchestrator should have allocated a /22 from the Vault pool before calling Provision")
			}
			allowInbound := spec.AzureAllowInboundCIDRs
			if len(allowInbound) == 0 {
				allowInbound = []string{"0.0.0.0/0"}
			}
			l4Vars := map[string]any{
				"subscription_id":              subscriptionID,
				"network_rg_name":              networkRG,
				"location":                     location,
				"name_prefix":                  "opord",
				"csa_id":                       spec.CSAID,
				"vnet_cidr":                    vnetCIDR,
				"allow_inbound_cidrs":          allowInbound,
				"flow_logs_storage_account_id": archiveStorageID,
				"tags":                         tags,
			}
			if _, err := p.runAccountLayerApply(ctx, p.secureVNetModDir, workspaceName(req.Workspace, "l4"), l4Vars, req.Credentials, req.Config, location); err != nil {
				return result, fmt.Errorf("L4 secure VNet: %w", err)
			}
			result.Layers["secure_vnet"] = "ready"
		}

		// -------- Layer 3: RBAC + Entra groups --------
		if !spec.Skip.AzureRBAC {
			l3Vars := map[string]any{
				"subscription_id":          subscriptionID,
				"subscription_resource_id": subscriptionResourceID,
				"name_prefix":              "opord",
				"csa_id":                   spec.CSAID,
				// Skip.AzureRBACGroups=true to create_groups=false: the layer
				// then makes only the custom role definitions (needs Owner ARM
				// role) and skips Entra group creation (which needs Graph
				// Group.ReadWrite.All / Groups Administrator the SP may lack).
				"create_groups": !spec.Skip.AzureRBACGroups,
				"tags":          tags,
			}
			if _, err := p.runAccountLayerApply(ctx, p.subscriptionRBACModDir, workspaceName(req.Workspace, "l3"), l3Vars, req.Credentials, req.Config, location); err != nil {
				return result, fmt.Errorf("L3 RBAC: %w", err)
			}
			result.Layers["rbac"] = "ready"
		}

		// -------- Companion: Azure Policy --------
		if !spec.Skip.AzurePolicy {
			allowedLocations := spec.AzureAllowedLocations
			if len(allowedLocations) == 0 {
				allowedLocations = []string{"westeurope", "northeurope"}
			}
			polVars := map[string]any{
				"subscription_id":          subscriptionID,
				"subscription_resource_id": subscriptionResourceID,
				"name_prefix":              "opord",
				"csa_id":                   spec.CSAID,
				"allowed_locations":        allowedLocations,
				"tags":                     tags,
			}
			if _, err := p.runAccountLayerApply(ctx, p.policyModDir, workspaceName(req.Workspace, "pol"), polVars, req.Credentials, req.Config, location); err != nil {
				return result, fmt.Errorf("Policy assignments: %w", err)
			}
			result.Layers["policy"] = "ready"
		}
	}

	return result, nil
}

// DestroyAccount tears down the layers in reverse order. Each tofu destroy
// runs in its own workspace and is idempotent; missing workspaces are not an
// error (Azure-side resources may already have been cleaned). L1 destroy in
// adopt mode is a no-op (we never owned the subscription).
func (p *Provider) DestroyAccount(ctx context.Context, req providers.AccountRequest) error {
	spec := req.Spec
	tags := tagsForAzureAccount(spec)
	location := spec.AzureLocation
	if location == "" {
		location = "westeurope"
	}
	subscriptionID := spec.AzureSubscriptionID
	subscriptionResourceID := "/subscriptions/" + subscriptionID

	// Reverse order: policy to L3 RBAC to L4 to L5 to KV to L2 to L1.

	if !spec.Skip.AzurePolicy {
		allowedLocations := spec.AzureAllowedLocations
		if len(allowedLocations) == 0 {
			allowedLocations = []string{"westeurope", "northeurope"}
		}
		_ = p.runAccountLayerDestroy(ctx, p.policyModDir, workspaceName(req.Workspace, "pol"), map[string]any{
			"subscription_id":          subscriptionID,
			"subscription_resource_id": subscriptionResourceID,
			"name_prefix":              "opord",
			"csa_id":                   spec.CSAID,
			"allowed_locations":        allowedLocations,
			"tags":                     tags,
		}, req.Credentials, req.Config, location)
	}
	if !spec.Skip.AzureRBAC {
		_ = p.runAccountLayerDestroy(ctx, p.subscriptionRBACModDir, workspaceName(req.Workspace, "l3"), map[string]any{
			"subscription_id":          subscriptionID,
			"subscription_resource_id": subscriptionResourceID,
			"name_prefix":              "opord",
			"csa_id":                   spec.CSAID,
			"tags":                     tags,
		}, req.Credentials, req.Config, location)
	}
	if !spec.Skip.AzureSecureVNet {
		// IMPORTANT: pass the REAL archive Storage Account id on destroy, not "".
		// The Flow Log resource references it; if tofu can't refresh the flow log
		// cleanly (empty SA id), the L4 destroy errors (swallowed below), the
		// VNet's active Flow Log blocks the VNet delete, and the network RG can
		// never be deleted by L2 - leaving an empty network-rg behind. The id is
		// deterministic from the naming convention, so reconstruct it.
		_ = p.runAccountLayerDestroy(ctx, p.secureVNetModDir, workspaceName(req.Workspace, "l4"), map[string]any{
			"subscription_id":              subscriptionID,
			"network_rg_name":              fmt.Sprintf("opord-%s-network-rg", spec.CSAID),
			"location":                     location,
			"name_prefix":                  "opord",
			"csa_id":                       spec.CSAID,
			"vnet_cidr":                    firstNonEmpty(spec.AzureVNetCIDR, "10.20.0.0/22"),
			"flow_logs_storage_account_id": archiveStorageAccountID(subscriptionID, spec.CSAID),
			"tags":                         tags,
		}, req.Credentials, req.Config, location)
	}
	if !spec.Skip.AzureSecurityHardening {
		_ = p.runAccountLayerDestroy(ctx, p.securityHardeningModDir, workspaceName(req.Workspace, "l5"), map[string]any{
			"subscription_id":          subscriptionID,
			"subscription_resource_id": subscriptionResourceID,
			"logs_rg_name":             fmt.Sprintf("opord-%s-logs-rg", spec.CSAID),
			"location":                 location,
			"name_prefix":              "opord",
			"csa_id":                   spec.CSAID,
			"tags":                     tags,
		}, req.Credentials, req.Config, location)
	}
	if !spec.Skip.AzureKeyVault {
		_ = p.runAccountLayerDestroy(ctx, p.keyVaultBaselineModDir, workspaceName(req.Workspace, "kv"), map[string]any{
			"subscription_id":  subscriptionID,
			"security_rg_name": fmt.Sprintf("opord-%s-security-rg", spec.CSAID),
			"location":         location,
			"name_prefix":      "opord",
			"csa_id":           spec.CSAID,
			"tags":             tags,
		}, req.Credentials, req.Config, location)
	}
	if !spec.Skip.Baseline {
		_ = p.runAccountLayerDestroy(ctx, p.subscriptionBaselineModDir, workspaceName(req.Workspace, "l2"), map[string]any{
			"subscription_id": subscriptionID,
			"location":        location,
			"name_prefix":     "opord",
			"csa_id":          spec.CSAID,
			"csa_cloud_name":  spec.CloudName,
			"tags":            tags,
		}, req.Credentials, req.Config, location)
	}
	// L1 destroy:
	//   - adopt mode: data-only resource, nothing to destroy (no-op).
	//   - create mode: NOT done by OPORD. Subscription cancellation is a
	//     90-day MCA process the operator initiates manually; documented in
	//     the decommission runbook.
	return nil
}

// workspaceName returns a deterministic per-layer workspace name so a re-run
// reuses the same state and a partial destroy targets the exact layer.
func workspaceName(base, layer string) string {
	return base + "-" + layer
}

// archiveStorageAccountID reconstructs the L5 archive Storage Account's ARM id
// from the naming convention (modules/azure-security-hardening): the SA name is
// lower(prefix+csa) stripped of non-alphanumerics + "logssa", capped to 24
// chars, in the logs RG. Used on destroy so the L4 Flow Log can refresh against
// its real storage account and tear down cleanly (see DestroyAccount L4 block).
func archiveStorageAccountID(subscriptionID, csaID string) string {
	if subscriptionID == "" || csaID == "" {
		return ""
	}
	raw := strings.ToLower("opord" + csaID)
	var b strings.Builder
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	name := b.String() + "logssa"
	if len(name) > 24 {
		name = name[:24]
	}
	return fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/opord-%s-logs-rg/providers/Microsoft.Storage/storageAccounts/%s",
		subscriptionID, csaID, name,
	)
}

// outString reads a string output (already json-encoded) from a tofu run.
func outString(outs map[string]json.RawMessage, key string) string {
	raw, ok := outs[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}
