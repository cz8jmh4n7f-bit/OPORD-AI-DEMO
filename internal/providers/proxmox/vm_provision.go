package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

func (p *Provider) writeVMVarsFile(req providers.VMRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildVMVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling vm vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-proxmox-vm-*.tfvars.json")
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

// ProvisionVM clones the VMs (tofu apply) for the request's workspace.
func (p *Provider) ProvisionVM(ctx context.Context, req providers.VMRequest) (*providers.VMResult, error) {
	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeVMVarsFile(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-proxmox-vm-*.tfplan")
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
		Names:      vmOutStrings(outs, "vm_names"),
		IDs:        vmOutStrings(outs, "vm_ids"),
		PrivateIPs: vmOutStrings(outs, "vm_ips"),
		RawOutputs: vmRawMap(outs),
	}, nil
}

// DestroyVM tears down the VMs for the request's workspace.
func (p *Provider) DestroyVM(ctx context.Context, req providers.VMRequest) error {
	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeVMVarsFile(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

func vmOutStrings(outs map[string]json.RawMessage, key string) []string {
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

func vmRawMap(outs map[string]json.RawMessage) map[string]any {
	m := make(map[string]any, len(outs))
	for k, v := range outs {
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			m[k] = val
		}
	}
	return m
}
