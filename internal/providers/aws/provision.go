package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

func (p *Provider) backendConfig() map[string]string {
	if p.cfg.StateConnStr == "" {
		return nil
	}
	return map[string]string{"conn_str": p.cfg.StateConnStr}
}

func (p *Provider) writeVMVars(req providers.VMRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildVMVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling vm vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-aws-vm-*.tfvars.json")
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

// ProvisionVM creates EC2 instances (tofu apply) for the request's workspace.
func (p *Provider) ProvisionVM(ctx context.Context, req providers.VMRequest) (*providers.VMResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	if err := p.setTargetEnv(ctx, r, req.Credentials, req.Config, req.Spec.Region, req.Spec.TargetAccount); err != nil {
		return nil, err
	}
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	// Deploy into a member account (ADR-0013): the VM needs a subnet that exists
	// THERE - the member's default VPC was removed by the account factory, so a
	// null subnet_id (the provider's own-account config) has nowhere to launch.
	// Discover one of the member's subnets (its secure VPC) and target it. No-op
	// when target_account is empty.
	cfg2, err := p.applyTargetSubnets(ctx, req.Credentials, req.Config, req.Spec.TargetAccount)
	if err != nil {
		return nil, err
	}
	if ids, ok := cfg2["subnet_ids"].([]string); ok && len(ids) > 0 {
		cfg2["subnet_id"] = ids[0] // the VM module takes a single subnet_id
	}
	req.Config = cfg2
	varsFile, cleanup, err := p.writeVMVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-aws-vm-*.tfplan")
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
	return &providers.VMResult{
		Names:      outStrings(outs, "vm_names"),
		IDs:        outStrings(outs, "vm_ids"),
		PrivateIPs: outStrings(outs, "private_ips"),
		PublicIPs:  outStrings(outs, "public_ips"),
		RawOutputs: rawMap(outs),
	}, nil
}

// DestroyVM tears down the EC2 instances for the request's workspace.
func (p *Provider) DestroyVM(ctx context.Context, req providers.VMRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
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
	if ids, ok := cfg2["subnet_ids"].([]string); ok && len(ids) > 0 {
		cfg2["subnet_id"] = ids[0]
	}
	req.Config = cfg2
	varsFile, cleanup, err := p.writeVMVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

func outStrings(outs map[string]json.RawMessage, key string) []string {
	raw, ok := outs[key]
	if !ok {
		return nil
	}
	var ss []string
	if err := json.Unmarshal(raw, &ss); err == nil {
		return ss
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]string, 0, len(arr))
		for _, e := range arr {
			out = append(out, fmt.Sprintf("%v", e))
		}
		return out
	}
	return nil
}

func rawMap(outs map[string]json.RawMessage) map[string]any {
	m := make(map[string]any, len(outs))
	for k, v := range outs {
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			m[k] = val
		}
	}
	return m
}
