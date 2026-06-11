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

// The k8s-shaped Provider methods wrap modules/azure-aks (a managed AKS
// cluster + system node pool). Like EKS, AKS is a managed control plane:
// Provision reports Managed=true so the orchestrator skips the kubeadm
// Phase 2 (Ansible) bootstrap.

func (p *Provider) prepareAKS(ctx context.Context, req providers.Request) (*tofu.Runner, error) {
	r := tofu.New(p.cfg.TofuBin, p.aksModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	return r, nil
}

func (p *Provider) writeAKSVars(req providers.Request) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildAKSVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling aks vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-aks-*.tfvars.json")
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

// Validate checks a cluster spec against AKS constraints. AKS is managed, so
// it needs neither a template nor static node IPs.
func (p *Provider) Validate(_ context.Context, spec models.ClusterSpec) error {
	var errs []string
	if spec.Workers.Count < 1 {
		errs = append(errs, "worker count must be >= 1 (AKS system node pool)")
	}
	// AKS rejects k8s versions that have rolled out of support; let the
	// provider surface the real error rather than maintain a static allowlist.
	if len(errs) > 0 {
		return fmt.Errorf("invalid aks cluster spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Preflight validates the var mapping + the azure-aks module offline.
func (p *Provider) Preflight(ctx context.Context, req providers.Request) (*providers.PreflightResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeAKSVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.aksModuleDir, p.log)
	if err := r.InitBackendless(ctx); err != nil {
		return nil, err
	}
	if err := r.Validate(ctx); err != nil {
		return nil, err
	}
	return &providers.PreflightResult{
		Workspace:   req.Workspace,
		ModuleValid: true,
		Summary: fmt.Sprintf("azure-aks validated; managed control plane k8s%s + %d-node system pool (%s)",
			versionTag(req.Spec.KubernetesVersion), req.Spec.Workers.Count, aksNodeSize(req)),
	}, nil
}

func (p *Provider) Plan(ctx context.Context, req providers.Request) (*providers.PlanResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r, err := p.prepareAKS(ctx, req)
	if err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeAKSVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	hasChanges, output, err := r.Plan(ctx, varsFile, "")
	if err != nil {
		return nil, err
	}
	return &providers.PlanResult{Summary: output, HasChanges: hasChanges}, nil
}

func (p *Provider) Provision(ctx context.Context, req providers.Request) (*providers.ProvisionResult, error) {
	// Deploy-into a OPORD-managed subscription (ADR-0013): override subscription_id
	// so the AKS cluster lands in the target subscription, using the provider's own
	// credentials. No-op when unset.
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r, err := p.prepareAKS(ctx, req)
	if err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeAKSVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-aks-*.tfplan")
	if err != nil {
		return nil, fmt.Errorf("creating plan file: %w", err)
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
	return parseAKSOutputs(outs, req), nil
}

func (p *Provider) Destroy(ctx context.Context, req providers.Request) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r, err := p.prepareAKS(ctx, req)
	if err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeAKSVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

func aksClusterName(req providers.Request) string {
	if n := strings.TrimSpace(req.Name); n != "" {
		return n
	}
	return "opord-" + safePrefix(req.Workspace, 12)
}

func aksNodeSize(req providers.Request) string {
	if mt := strings.TrimSpace(req.Spec.MachineType); mt != "" {
		return mt
	}
	return cfgStringDefault(req.Config, "aks_node_vm_size", "Standard_B2s")
}

func versionTag(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return " (AKS default)"
	}
	return " " + v
}

// buildAKSVars maps a Request onto the modules/azure-aks OpenTofu inputs.
func buildAKSVars(req providers.Request) map[string]any {
	spec := req.Spec
	cfg := req.Config

	location := cfgString(cfg, "location")
	if location == "" {
		location = "westeurope"
	}

	count := spec.Workers.Count
	if count < 1 {
		count = 1
	}

	diskGB := spec.Workers.Specs.DiskGB
	if diskGB <= 0 {
		diskGB = 30
	}

	namePrefix := aksClusterName(req)

	return map[string]any{
		"location":           location,
		"name_prefix":        namePrefix,
		"environment":        cfgStringDefault(cfg, "environment", "dev"),
		"kubernetes_version": spec.KubernetesVersion,
		"node_count":         count,
		"node_vm_size":       aksNodeSize(req),
		"node_disk_gb":       diskGB,
	}
}

func parseAKSOutputs(outs map[string]json.RawMessage, req providers.Request) *providers.ProvisionResult {
	endpoint := azureOutString(outs, "fqdn")
	clusterName := azureOutString(outs, "cluster_name")
	if clusterName == "" {
		clusterName = aksClusterName(req)
	}
	rgName := azureOutString(outs, "resource_group_name")

	// AKS exposes the kubeconfig as a raw output (kube_config). Keep it in the
	// raw bag so callers (orchestrator) can persist it to disk; surface the `az`
	// command in the Kubeconfig field as a fallback for users who'd prefer it.
	kubeconfigCmd := fmt.Sprintf("az aks get-credentials --resource-group %s --name %s", rgName, clusterName)
	return &providers.ProvisionResult{
		Nodes:                nil, // managed system pool; nodes join dynamically
		ControlPlaneEndpoint: endpoint,
		Managed:              true,
		Kubeconfig:           kubeconfigCmd,
		RawOutputs:           rawMap(outs),
	}
}
