package vsphere

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

var _ providers.VMProvisioner = (*Provider)(nil)

func (p *Provider) vmModuleDir() string {
	return filepath.Join(p.cfg.ModulesDir, "vsphere-vm")
}

// buildVMVars maps a VMRequest onto the modules/vsphere-vm OpenTofu inputs.
func buildVMVars(req providers.VMRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	creds := req.Credentials

	dataDisks := spec.DataDisksGB
	if dataDisks == nil {
		dataDisks = []int{}
	}

	return map[string]any{
		"vsphere_server":               cfgString(cfg, "server"),
		"vsphere_user":                 creds["user"],
		"vsphere_password":             creds["password"],
		"vsphere_allow_unverified_ssl": cfgBool(cfg, "allow_unverified_ssl", true),
		"vsphere_datacenter":           cfgString(cfg, "datacenter"),
		"vsphere_cluster":              cfgString(cfg, "cluster"),
		"vsphere_datastore":            cfgString(cfg, "datastore"),
		"vsphere_network":              cfgString(cfg, "network"),
		"vsphere_folder_path":          cfgString(cfg, "folder"),
		"template_name":                spec.Template,
		"vm_count":                     spec.Count,
		"name_prefix":                  spec.NamePrefix,
		"specs": map[string]any{
			"cpu":    spec.CPU,
			"memory": spec.MemoryMB,
			"disk":   spec.DiskGB,
		},
		"data_disks":     dataDisks,
		"ip_start":       spec.IPStart,
		"netmask_bits":   netmaskToBits(spec.Netmask),
		"gateway":        spec.Gateway,
		"dns_servers":    spec.DNSServers,
		"dns_suffix":     spec.DNSSuffix,
		"ssh_user":       spec.SSHUser,
		"ssh_public_key": spec.SSHPublicKey,
	}
}

// PreflightVM validates the vm var mapping + the vsphere-vm module offline.
func (p *Provider) PreflightVM(ctx context.Context, req providers.VMRequest) error {
	data, err := json.Marshal(buildVMVars(req))
	if err != nil {
		return fmt.Errorf("marshaling vm vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-vsphere-vm-*.tfvars.json")
	if err != nil {
		return fmt.Errorf("creating vars file: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing vars file: %w", err)
	}
	_ = f.Close()

	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir(), p.log)
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}
