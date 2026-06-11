package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// LoadBalancerProvisioner: an AWS Application Load Balancer (+ listener(s) +
// target group) via modules/aws-alb. VPC-bound the same way RDS is - for a
// deploy-into-member request the subnets are discovered in the target account
// (applyTargetSubnets, ADR-0013).

var _ providers.LoadBalancerProvisioner = (*Provider)(nil)

func (p *Provider) albModuleDir() string {
	return filepath.Join(p.cfg.ModulesDir, "aws-alb")
}

func (p *Provider) writeLBVars(req providers.LoadBalancerRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildLBVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling loadbalancer vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-alb-*.tfvars.json")
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

// PreflightLoadBalancer validates the var mapping + the aws-alb module offline.
func (p *Provider) PreflightLoadBalancer(ctx context.Context, req providers.LoadBalancerRequest) error {
	_, cleanup, err := p.writeLBVars(req)
	if err != nil {
		return err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.albModuleDir(), p.log)
	r.SetEnv(awsTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionLoadBalancer creates the ALB (tofu apply) for the request's workspace.
func (p *Provider) ProvisionLoadBalancer(ctx context.Context, req providers.LoadBalancerRequest) (*providers.LoadBalancerResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.albModuleDir(), p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, req.Spec.Region, req.Spec.TargetAccount); err != nil {
		return nil, err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	// Deploy into a member account: the ALB needs subnets that exist THERE, not
	// the provider's own-account subnets (ADR-0013). No-op for the provider's account.
	cfg2, err := p.applyTargetSubnets(ctx, req.Credentials, req.Config, req.Spec.TargetAccount)
	if err != nil {
		return nil, err
	}
	req.Config = cfg2
	varsFile, cleanup, err := p.writeLBVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-aws-alb-*.tfplan")
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
	return &providers.LoadBalancerResult{
		DNSName:        dbOutString(outs, "dns_name"),
		ARN:            dbOutString(outs, "arn"),
		ZoneID:         dbOutString(outs, "zone_id"),
		TargetGroupARN: dbOutString(outs, "target_group_arn"),
		RawOutputs:     rawMap(outs),
	}, nil
}

// DestroyLoadBalancer tears down the ALB for the request's workspace.
func (p *Provider) DestroyLoadBalancer(ctx context.Context, req providers.LoadBalancerRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.albModuleDir(), p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, req.Spec.Region, req.Spec.TargetAccount); err != nil {
		return err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	cfg2, err := p.applyTargetSubnets(ctx, req.Credentials, req.Config, req.Spec.TargetAccount)
	if err != nil {
		return err
	}
	req.Config = cfg2
	varsFile, cleanup, err := p.writeLBVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildLBVars maps a LoadBalancerRequest onto the modules/aws-alb OpenTofu inputs.
func buildLBVars(req providers.LoadBalancerRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := req.Name
	if name == "" {
		name = spec.Name
	}
	subnets := spec.SubnetIDs
	if len(subnets) == 0 {
		subnets = cfgStringSlice(cfg, "subnet_ids")
	}
	targetType := spec.TargetType
	if targetType == "" {
		targetType = "instance"
	}
	healthCheckPath := spec.HealthCheckPath
	if healthCheckPath == "" {
		healthCheckPath = "/"
	}
	vars := map[string]any{
		"region":            firstNonEmpty(spec.Region, cfgString(cfg, "region")),
		"name":              name,
		"internal":          spec.Internal,
		"target_type":       targetType,
		"health_check_path": healthCheckPath,
		"tags": map[string]string{
			"opord:kind":      "loadbalancer",
			"opord:workspace": req.Workspace,
		},
	}
	if len(subnets) > 0 {
		vars["subnet_ids"] = subnets
	}
	if len(spec.SecurityGroupIDs) > 0 {
		vars["security_group_ids"] = spec.SecurityGroupIDs
	}
	if len(spec.Targets) > 0 {
		vars["targets"] = spec.Targets
	}
	// Omit listeners when empty so the module default (HTTP:80) applies.
	if len(spec.Listeners) > 0 {
		ls := make([]map[string]any, 0, len(spec.Listeners))
		for _, l := range spec.Listeners {
			ls = append(ls, map[string]any{
				"port":            l.Port,
				"protocol":        l.Protocol,
				"certificate_arn": l.CertificateARN,
			})
		}
		vars["listeners"] = ls
	}
	return vars
}
