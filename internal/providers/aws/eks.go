package aws

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

// The k8s-shaped Provider methods wrap modules/aws-eks (a managed EKS control
// plane + node group). Unlike kubeadm-on-VMs, there is no Phase 2 Ansible step:
// Provision reports Managed=true so the orchestrator skips bootstrap.

func (p *Provider) prepareEKS(ctx context.Context, req providers.Request) (*tofu.Runner, error) {
	r := tofu.New(p.cfg.TofuBin, p.eksModuleDir, p.log)
	// Deploy-into a OPORD-managed member account (ADR-0013): AssumeRole into it when
	// target_account is set; plain provider env otherwise. Clusters carry no
	// spec.Region, so pass "" (the env mapper falls back to the provider config region).
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, "", req.Spec.TargetAccount); err != nil {
		return nil, err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	return r, nil
}

func (p *Provider) writeEKSVars(req providers.Request) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildEKSVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling eks vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-eks-*.tfvars.json")
	if err != nil {
		return "", noop, fmt.Errorf("creating vars file: %w", err)
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

// Validate checks a cluster spec against EKS constraints. EKS is a managed
// control plane, so it needs neither a template nor static node IPs.
func (p *Provider) Validate(_ context.Context, spec models.ClusterSpec) error {
	var errs []string
	if strings.TrimSpace(spec.KubernetesVersion) == "" {
		errs = append(errs, "kubernetes_version is required (e.g. \"1.31\")")
	}
	if spec.Workers.Count < 1 {
		errs = append(errs, "worker count must be >= 1")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid eks cluster spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Preflight validates the spec mapping + the aws-eks module offline.
func (p *Provider) Preflight(ctx context.Context, req providers.Request) (*providers.PreflightResult, error) {
	_, cleanup, err := p.writeEKSVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.eksModuleDir, p.log)
	if err := r.InitBackendless(ctx); err != nil {
		return nil, err
	}
	if err := r.Validate(ctx); err != nil {
		return nil, err
	}
	return &providers.PreflightResult{
		Workspace:   req.Workspace,
		ModuleValid: true,
		Summary: fmt.Sprintf("aws-eks validated; managed control plane k8s v%s + %d-node group (%s)",
			req.Spec.KubernetesVersion, req.Spec.Workers.Count, eksInstanceType(req)),
	}, nil
}

func (p *Provider) Plan(ctx context.Context, req providers.Request) (*providers.PlanResult, error) {
	r, err := p.prepareEKS(ctx, req)
	if err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeEKSVars(req)
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
	r, err := p.prepareEKS(ctx, req)
	if err != nil {
		return nil, err
	}
	// Use subnets in the member account's VPC for the EKS control plane + node group
	// (the provider's own subnet_ids live in the provider account). No-op without a target.
	cfg2, err := p.applyTargetSubnets(ctx, req.Credentials, req.Config, req.Spec.TargetAccount)
	if err != nil {
		return nil, err
	}
	req.Config = cfg2
	varsFile, cleanup, err := p.writeEKSVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-aws-eks-*.tfplan")
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
	return parseEKSOutputs(outs, req), nil
}

func (p *Provider) Destroy(ctx context.Context, req providers.Request) error {
	r, err := p.prepareEKS(ctx, req)
	if err != nil {
		return err
	}
	cfg2, err := p.applyTargetSubnets(ctx, req.Credentials, req.Config, req.Spec.TargetAccount)
	if err != nil {
		return err
	}
	req.Config = cfg2
	varsFile, cleanup, err := p.writeEKSVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

func eksClusterName(req providers.Request) string {
	if n := strings.TrimSpace(req.Name); n != "" {
		return n
	}
	return "opord-" + req.Workspace
}

func eksInstanceType(req providers.Request) string {
	if mt := strings.TrimSpace(req.Spec.MachineType); mt != "" {
		return mt
	}
	return cfgStringDefault(req.Config, "node_instance_type", "t3.medium")
}

// buildEKSVars maps a Request onto the modules/aws-eks OpenTofu inputs.
func buildEKSVars(req providers.Request) map[string]any {
	spec := req.Spec
	cfg := req.Config

	desired := spec.Workers.Count
	if desired < 1 {
		desired = 1
	}
	return map[string]any{
		"region":             cfgString(cfg, "region"),
		"cluster_name":       eksClusterName(req),
		"kubernetes_version": spec.KubernetesVersion,
		"subnet_ids":         cfgStringSlice(cfg, "subnet_ids"),
		"node_instance_type": eksInstanceType(req),
		"node_desired_size":  desired,
		"node_min_size":      1,
		"node_max_size":      desired,
	}
}

func parseEKSOutputs(outs map[string]json.RawMessage, req providers.Request) *providers.ProvisionResult {
	endpoint := jsonString(outs, "cluster_endpoint")
	clusterName := jsonString(outs, "cluster_name")
	if clusterName == "" {
		clusterName = eksClusterName(req)
	}
	region := jsonString(outs, "region")
	if region == "" {
		region = cfgString(req.Config, "region")
	}

	raw := make(map[string]any, len(outs))
	for k, v := range outs {
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			raw[k] = val
		}
	}

	// EKS kubeconfig is fetched via the AWS CLI (tokens are short-lived), so we
	// record the command rather than a static file.
	kubeconfig := fmt.Sprintf("aws eks update-kubeconfig --name %s --region %s", clusterName, region)

	return &providers.ProvisionResult{
		Nodes:                nil, // managed node group; nodes join dynamically
		ControlPlaneEndpoint: endpoint,
		Managed:              true,
		Kubeconfig:           kubeconfig,
		RawOutputs:           raw,
	}
}

func jsonString(outs map[string]json.RawMessage, key string) string {
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
