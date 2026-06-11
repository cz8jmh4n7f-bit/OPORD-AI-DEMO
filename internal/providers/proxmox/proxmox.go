// Package proxmox implements an infrastructure provider for Proxmox VE. It
// currently supports the standalone-VM blueprint (modules/proxmox-vm via the
// bpg/proxmox OpenTofu provider). Kubernetes-cluster provisioning is not
// implemented yet, so the k8s-shaped Provider methods return a clear error.
package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// Config configures the Proxmox provider.
type Config struct {
	ModulesDir   string
	TofuBin      string
	StateConnStr string
	Logger       *slog.Logger
}

// Provider wraps the Proxmox OpenTofu modules.
type Provider struct {
	cfg          Config
	vmModuleDir  string
	k8sModuleDir string
	log          *slog.Logger
}

var (
	_ providers.Provider      = (*Provider)(nil)
	_ providers.VMProvisioner = (*Provider)(nil)
)

// New constructs a Proxmox provider.
func New(cfg Config) *Provider {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Provider{
		cfg:          cfg,
		vmModuleDir:  filepath.Join(cfg.ModulesDir, "proxmox-vm"),
		k8sModuleDir: filepath.Join(cfg.ModulesDir, "proxmox-k8s"),
		log:          log,
	}
}

// Register adds the Proxmox provider factory to a registry.
func Register(reg *providers.Registry, cfg Config) {
	reg.Register(models.ProviderProxmox, func() providers.Provider { return New(cfg) })
}

func (p *Provider) Type() models.ProviderType { return models.ProviderProxmox }

// Kubernetes-cluster Provider methods (Validate/Preflight/Plan/Provision/Destroy)
// wrap modules/proxmox-k8s - see k8s.go.

// PreflightVM validates the vm var mapping + the proxmox-vm module offline.
func (p *Provider) PreflightVM(ctx context.Context, req providers.VMRequest) error {
	data, err := json.Marshal(buildVMVars(req))
	if err != nil {
		return fmt.Errorf("marshaling vm vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-proxmox-vm-*.tfvars.json")
	if err != nil {
		return fmt.Errorf("creating vars file: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing vars file: %w", err)
	}
	_ = f.Close()

	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// buildVMVars maps a VMRequest onto the modules/proxmox-vm OpenTofu inputs.
func buildVMVars(req providers.VMRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	creds := req.Credentials

	return map[string]any{
		"proxmox_endpoint": proxmoxEndpoint(cfg),
		"proxmox_username": creds["user"],
		"proxmox_password": creds["password"],
		"proxmox_insecure": cfgBool(cfg, "insecure", true),
		"node_name":        cfgString(cfg, "node"),
		"datastore_id":     cfgStringDefault(cfg, "datastore", "local-lvm"),
		"network_bridge":   cfgStringDefault(cfg, "bridge", "vmbr0"),
		"template_vmid":    templateVMID(cfg, spec.Template),
		"vm_count":         spec.Count,
		"name_prefix":      spec.NamePrefix,
		"cores":            spec.CPU,
		"memory_mb":        spec.MemoryMB,
		"disk_gb":          spec.DiskGB,
		"ip_start":         spec.IPStart,
		"netmask_bits":     netmaskBits(spec.Netmask),
		"gateway":          spec.Gateway,
		"dns_servers":      spec.DNSServers,
		"dns_domain":       spec.DNSSuffix,
		"ssh_user":         spec.SSHUser,
		"ssh_public_key":   spec.SSHPublicKey,
	}
}

// proxmoxEndpoint resolves the Proxmox API URL, tolerating both the canonical
// "endpoint" key and "server" (what `opord provider add` and the web Add-provider
// form record), so a provider added through any path provisions correctly.
func proxmoxEndpoint(cfg map[string]any) string {
	if e := cfgString(cfg, "endpoint"); e != "" {
		return e
	}
	return cfgString(cfg, "server")
}

func cfgString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	if v, ok := cfg[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func cfgStringDefault(cfg map[string]any, key, def string) string {
	if s := cfgString(cfg, key); s != "" {
		return s
	}
	return def
}

func cfgBool(cfg map[string]any, key string, def bool) bool {
	if cfg == nil {
		return def
	}
	if v, ok := cfg[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
		if s, ok := v.(string); ok {
			if b, err := strconv.ParseBool(s); err == nil {
				return b
			}
		}
	}
	return def
}

// templateVMID resolves the Proxmox template VMID from config["template_vmid"]
// or a numeric spec.Template; defaults to 0 (preflight validate does not use it).
func templateVMID(cfg map[string]any, template string) int {
	if cfg != nil {
		switch v := cfg["template_vmid"].(type) {
		case float64:
			return int(v)
		case int:
			return v
		case string:
			if n, err := strconv.Atoi(v); err == nil {
				return n
			}
		}
	}
	if n, err := strconv.Atoi(strings.TrimSpace(template)); err == nil {
		return n
	}
	return 0
}

func netmaskBits(s string) int {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "/"))
	if s == "" {
		return 24
	}
	if strings.Contains(s, ".") {
		bits := 0
		for _, part := range strings.Split(s, ".") {
			n, err := strconv.Atoi(part)
			if err != nil || n < 0 || n > 255 {
				return 24
			}
			for n > 0 {
				bits += n & 1
				n >>= 1
			}
		}
		return bits
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 32 {
		return 24
	}
	return n
}
