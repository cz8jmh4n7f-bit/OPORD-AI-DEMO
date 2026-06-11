package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// VMProvisioner: Linux VM via modules/azure-vm (azurerm_linux_virtual_machine
// in a fresh resource group with a NIC, optional Public IP and a locked NSG).

func (p *Provider) writeVMVars(req providers.VMRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildVMVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling azure vm vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-vm-*.tfvars.json")
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

// PreflightVM validates the azure-vm module + var mapping offline (no API calls).
func (p *Provider) PreflightVM(ctx context.Context, req providers.VMRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeVMVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, req.Spec.Region))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionVM creates the VM(s) (tofu apply) for the request's workspace.
func (p *Provider) ProvisionVM(ctx context.Context, req providers.VMRequest) (*providers.VMResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, req.Spec.Region))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeVMVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-vm-*.tfplan")
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

// DestroyVM tears down the VM(s) for the request's workspace.
func (p *Provider) DestroyVM(ctx context.Context, req providers.VMRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, req.Spec.Region))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeVMVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildVMVars maps a VMRequest (provider-neutral VMSpec + creds + config) onto
// the modules/azure-vm inputs. Default image: Ubuntu 22.04 LTS gen2.
func buildVMVars(req providers.VMRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config

	location := spec.Region
	if location == "" {
		location = cfgString(cfg, "location")
	}
	if location == "" {
		location = "westeurope"
	}

	namePrefix := spec.NamePrefix
	if namePrefix == "" {
		// Workspace is a UUID; trim it to keep the name short and valid for
		// Azure naming (resource group, VNet names accept up to 90 chars but
		// VM names are more restrictive).
		namePrefix = "opord-" + safePrefix(req.Workspace, 12)
	}

	count := spec.Count
	if count <= 0 {
		count = 1
	}

	size := spec.InstanceType
	if size == "" {
		size = "Standard_B1s"
	}

	sshUser := spec.SSHUser
	if sshUser == "" {
		sshUser = "azureuser"
	}

	diskGB := spec.DiskGB
	if diskGB <= 0 {
		diskGB = 30
	}

	publicIP := spec.PublicIP
	if azureIsProd(cfg) {
		publicIP = false
	}
	sshSourcesDefault := []string{"0.0.0.0/0"}
	if azureIsProd(cfg) {
		sshSourcesDefault = []string{"10.0.0.0/8"}
	}

	return map[string]any{
		"location":            location,
		"name_prefix":         namePrefix,
		"environment":         cfgStringDefault(cfg, "environment", "dev"),
		"vm_count":            count,
		"vm_size":             size,
		"admin_username":      sshUser,
		"ssh_public_key":      spec.SSHPublicKey,
		"os_disk_gb":          diskGB,
		"associate_public_ip": publicIP,
		"allow_ssh_from":      cfgStringListDefault(cfg, "azure_allow_ssh_from", sshSourcesDefault),
	}
}

// safePrefix trims a string to maxLen and removes characters that would be
// rejected by Azure resource naming.
func safePrefix(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-':
			out = append(out, c)
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32) // to lower
		}
	}
	if len(out) == 0 {
		return "vm"
	}
	return string(out)
}

// outStrings unmarshals a tofu list-of-strings output. Returns nil for missing
// or unparsable values (caller treats nil as "no IPs/IDs yet").
func outStrings(outs map[string]json.RawMessage, key string) []string {
	raw, ok := outs[key]
	if !ok {
		return nil
	}
	var ss []string
	if err := json.Unmarshal(raw, &ss); err == nil {
		return ss
	}
	return nil
}

// rawMap returns every output as a generic any (used as a passthrough for
// downstream code that wants the full output set).
func rawMap(outs map[string]json.RawMessage) map[string]any {
	if len(outs) == 0 {
		return nil
	}
	m := make(map[string]any, len(outs))
	for k, v := range outs {
		var x any
		if err := json.Unmarshal(v, &x); err == nil {
			m[k] = x
		}
	}
	return m
}
