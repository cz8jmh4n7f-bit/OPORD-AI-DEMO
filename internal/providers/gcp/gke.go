package gcp

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

// The k8s-shaped Provider methods wrap modules/gcp-gke (a managed GKE cluster +
// a node pool). Like EKS/AKS, GKE is a managed control plane: Provision reports
// Managed=true so the orchestrator skips the kubeadm Phase 2 (Ansible) bootstrap;
// the kubeconfig is the `gcloud container clusters get-credentials` command.

func (p *Provider) prepareGKE(ctx context.Context, req providers.Request) (*tofu.Runner, error) {
	r := tofu.New(p.cfg.TofuBin, p.gkeModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	return r, nil
}

func (p *Provider) writeGKEVars(req providers.Request) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildGKEVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gke vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-gke-*.tfvars.json")
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

// Validate checks a cluster spec against GKE constraints. GKE is managed, so it
// needs neither a template nor static node IPs.
func (p *Provider) Validate(_ context.Context, spec models.ClusterSpec) error {
	if spec.Workers.Count < 1 {
		return fmt.Errorf("invalid gke cluster spec: worker count must be >= 1 (node pool)")
	}
	return nil
}

// Preflight validates the var mapping + the gcp-gke module offline.
func (p *Provider) Preflight(ctx context.Context, req providers.Request) (*providers.PreflightResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeGKEVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.gkeModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return nil, err
	}
	if err := r.Validate(ctx); err != nil {
		return nil, err
	}
	count := req.Spec.Workers.Count
	if count < 1 {
		count = 1
	}
	return &providers.PreflightResult{
		Workspace:   req.Workspace,
		ModuleValid: true,
		Summary: fmt.Sprintf("gcp-gke validated; managed control plane k8s%s + %d-node pool (%s)",
			gkeVersionTag(req.Spec.KubernetesVersion), count, gkeMachineType(req)),
	}, nil
}

func (p *Provider) Plan(ctx context.Context, req providers.Request) (*providers.PlanResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r, err := p.prepareGKE(ctx, req)
	if err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeGKEVars(req)
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
	// Deploy-into a OPORD-managed project (ADR-0013): override project_id so the
	// GKE cluster (and its kubeconfig command) land in the target project, using
	// the provider's own folder-inherited credentials. No-op when unset.
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	// A governed project has no "default" network - point GKE at the factory VPC + subnet.
	req.Config = p.withGKENetwork(ctx, req.Config, req.Credentials, req.Spec.TargetAccount)
	r, err := p.prepareGKE(ctx, req)
	if err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeGKEVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-gcp-gke-*.tfplan")
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
	return parseGKEOutputs(outs, req), nil
}

func (p *Provider) Destroy(ctx context.Context, req providers.Request) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	req.Config = p.withGKENetwork(ctx, req.Config, req.Credentials, req.Spec.TargetAccount)
	r, err := p.prepareGKE(ctx, req)
	if err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeGKEVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

func gkeClusterName(req providers.Request) string {
	if n := strings.TrimSpace(req.Name); n != "" {
		return safeName(n, 40)
	}
	return "opord-" + safeName(req.Workspace, 12)
}

func gkeMachineType(req providers.Request) string {
	if mt := strings.TrimSpace(req.Spec.MachineType); mt != "" {
		return mt
	}
	return cfgStringDefault(req.Config, "gke_machine_type", "e2-small")
}

func gkeVersionTag(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return " (GKE default)"
	}
	return " " + v
}

// buildGKEVars maps a Request onto the modules/gcp-gke OpenTofu inputs.
func buildGKEVars(req providers.Request) map[string]any {
	spec := req.Spec
	cfg := req.Config

	region := cfgStringDefault(cfg, "region", "europe-west1")
	count := spec.Workers.Count
	if count < 1 {
		count = 1
	}
	diskGB := spec.Workers.Specs.DiskGB
	if diskGB <= 0 {
		diskGB = 30
	}

	return map[string]any{
		"name":               gkeClusterName(req),
		"region":             region,
		"zone":               cfgString(cfg, "zone"),
		"kubernetes_version": spec.KubernetesVersion,
		"node_count":         count,
		"machine_type":       gkeMachineType(req),
		"disk_gb":            diskGB,
		"environment":        cfgStringDefault(cfg, "environment", "dev"),
		"network":            cfgString(cfg, "network"),
		"subnetwork":         cfgString(cfg, "subnetwork"),
		"cni":                spec.CNI,
	}
}

// withGKENetwork discovers the factory VPC + a regional subnet when deploying a GKE
// cluster INTO a governed project (which has no "default" network) and adds them to
// the config so the module sets network/subnetwork explicitly. No-op without a target
// (or when discovery fails - e.g. keyless ADC has no in-process token).
func (p *Provider) withGKENetwork(ctx context.Context, cfg map[string]any, creds map[string]string, targetAccount string) map[string]any {
	if targetAccount == "" {
		return cfg
	}
	net := factoryNetwork(ctx, creds, targetAccount)
	if net == "" {
		return cfg
	}
	// Find a subnet that belongs to this VPC (and its region) - the factory VPC's
	// subnets live in the account's region, which may differ from the provider's
	// default region, so we align the cluster region to the subnet we found.
	vpcName := net[strings.LastIndex(net, "/")+1:]
	subnet, region := factorySubnet(ctx, creds, targetAccount, vpcName)
	out := make(map[string]any, len(cfg)+4)
	for k, v := range cfg {
		out[k] = v
	}
	out["network"] = net
	if subnet != "" {
		out["subnetwork"] = subnet
		out["region"] = region
		// Clear any provider-default zone (e.g. us-central1-a) so the location
		// derives from the factory region (<region>-b) and matches the subnet.
		out["zone"] = ""
	}
	return out
}

func parseGKEOutputs(outs map[string]json.RawMessage, req providers.Request) *providers.ProvisionResult {
	endpoint := outString(outs, "endpoint")
	clusterName := outString(outs, "cluster_name")
	if clusterName == "" {
		clusterName = gkeClusterName(req)
	}
	location := outString(outs, "location")
	project := cfgString(req.Config, "project_id")

	// GKE exposes the endpoint + CA in the raw bag; the get-credentials command is
	// the most reliable kubeconfig for users (it writes a token-auth context).
	kubeconfigCmd := fmt.Sprintf("gcloud container clusters get-credentials %s --zone %s", clusterName, location)
	if project != "" {
		kubeconfigCmd += " --project " + project
	}
	return &providers.ProvisionResult{
		Nodes:                nil, // managed node pool; nodes join dynamically
		ControlPlaneEndpoint: endpoint,
		Managed:              true,
		Kubeconfig:           kubeconfigCmd,
		RawOutputs:           rawMap(outs),
	}
}
