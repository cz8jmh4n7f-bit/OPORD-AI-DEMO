package proxmox

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

func (p *Provider) backendConfig() map[string]string {
	if p.cfg.StateConnStr == "" {
		return nil
	}
	return map[string]string{"conn_str": p.cfg.StateConnStr}
}

func (p *Provider) prepare(ctx context.Context, workspace string) (*tofu.Runner, error) {
	r := tofu.New(p.cfg.TofuBin, p.k8sModuleDir, p.log)
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return nil, err
	}
	return r, nil
}

func (p *Provider) writeClusterVars(req providers.Request) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildClusterVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling cluster vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-proxmox-k8s-*.tfvars.json")
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

// Validate checks a cluster spec against Proxmox/Kubernetes constraints.
func (p *Provider) Validate(_ context.Context, spec models.ClusterSpec) error {
	var errs []string
	if strings.TrimSpace(spec.Template) == "" {
		errs = append(errs, "template is required")
	}
	if spec.ControlPlane.Count < 1 || spec.ControlPlane.Count%2 == 0 {
		errs = append(errs, "control plane count must be an odd number >= 1")
	}
	if spec.Workers.Count < 1 {
		errs = append(errs, "worker count must be >= 1")
	}
	if strings.TrimSpace(spec.ControlPlane.IPStart) == "" || strings.TrimSpace(spec.Workers.IPStart) == "" {
		errs = append(errs, "control plane and worker ip_start are required")
	}
	if strings.TrimSpace(spec.Networking.Gateway) == "" {
		errs = append(errs, "networking.gateway is required")
	}
	if strings.TrimSpace(spec.Networking.ControlPlaneEndpoint) == "" {
		errs = append(errs, "networking.control_plane_endpoint is required")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid proxmox cluster spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Preflight validates the spec mapping + the proxmox-k8s module offline.
func (p *Provider) Preflight(ctx context.Context, req providers.Request) (*providers.PreflightResult, error) {
	_, cleanup, err := p.writeClusterVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	r := tofu.New(p.cfg.TofuBin, p.k8sModuleDir, p.log)
	if err := r.InitBackendless(ctx); err != nil {
		return nil, err
	}
	if err := r.Validate(ctx); err != nil {
		return nil, err
	}
	return &providers.PreflightResult{
		Workspace:   req.Workspace,
		ModuleValid: true,
		Summary: fmt.Sprintf("proxmox-k8s validated; %d control-plane + %d worker node(s), template %q, k8s v%s",
			req.Spec.ControlPlane.Count, req.Spec.Workers.Count, req.Spec.Template, req.Spec.KubernetesVersion),
	}, nil
}

func (p *Provider) Plan(ctx context.Context, req providers.Request) (*providers.PlanResult, error) {
	r, err := p.prepare(ctx, req.Workspace)
	if err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeClusterVars(req)
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
	r, err := p.prepare(ctx, req.Workspace)
	if err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeClusterVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-proxmox-k8s-*.tfplan")
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
	return parseClusterOutputs(outs)
}

func (p *Provider) Destroy(ctx context.Context, req providers.Request) error {
	r, err := p.prepare(ctx, req.Workspace)
	if err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeClusterVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildClusterVars maps a Request onto the modules/proxmox-k8s OpenTofu inputs.
func buildClusterVars(req providers.Request) map[string]any {
	spec := req.Spec
	cfg := req.Config
	creds := req.Credentials

	port := spec.Networking.ControlPlaneEndpointPort
	if port == 0 {
		port = 6443
	}

	return map[string]any{
		"proxmox_endpoint": proxmoxEndpoint(cfg),
		"proxmox_username": creds["user"],
		"proxmox_password": creds["password"],
		"proxmox_insecure": cfgBool(cfg, "insecure", true),
		"node_name":        cfgString(cfg, "node"),
		"datastore_id":     cfgStringDefault(cfg, "datastore", "local-lvm"),
		"network_bridge":   cfgStringDefault(cfg, "bridge", "vmbr0"),
		"template_vmid":    templateVMID(cfg, spec.Template),

		"control_plane_count": spec.ControlPlane.Count,
		"worker_count":        spec.Workers.Count,
		"control_plane_specs": map[string]any{
			"cpu":    spec.ControlPlane.Specs.CPU,
			"memory": spec.ControlPlane.Specs.MemoryMB,
			"disk":   spec.ControlPlane.Specs.DiskGB,
		},
		"worker_specs": map[string]any{
			"cpu":    spec.Workers.Specs.CPU,
			"memory": spec.Workers.Specs.MemoryMB,
			"disk":   spec.Workers.Specs.DiskGB,
		},
		"cp_name_prefix":              spec.ControlPlane.NamePrefix,
		"worker_name_prefix":          spec.Workers.NamePrefix,
		"cp_ip_start":                 spec.ControlPlane.IPStart,
		"worker_ip_start":             spec.Workers.IPStart,
		"netmask_bits":                netmaskBits(spec.Networking.Netmask),
		"gateway":                     spec.Networking.Gateway,
		"dns_servers":                 spec.Networking.DNSServers,
		"dns_domain":                  spec.Networking.DNSSuffix,
		"control_plane_endpoint":      spec.Networking.ControlPlaneEndpoint,
		"control_plane_endpoint_port": port,
		"ssh_user":                    spec.SSHUser,
		"ssh_public_key":              spec.SSHPublicKey,
	}
}

func parseClusterOutputs(outs map[string]json.RawMessage) (*providers.ProvisionResult, error) {
	cpIPs, _ := decodeStringSlice(outs, "control_plane_ips")
	cpNames, _ := decodeStringSlice(outs, "control_plane_names")
	wkIPs, _ := decodeStringSlice(outs, "worker_ips")
	wkNames, _ := decodeStringSlice(outs, "worker_names")

	nodes := make([]models.Node, 0, len(cpNames)+len(wkNames))
	for i, name := range cpNames {
		n := models.Node{Name: name, Role: models.RoleControlPlane}
		if i < len(cpIPs) {
			n.IPAddress = cpIPs[i]
		}
		nodes = append(nodes, n)
	}
	for i, name := range wkNames {
		n := models.Node{Name: name, Role: models.RoleWorker}
		if i < len(wkIPs) {
			n.IPAddress = wkIPs[i]
		}
		nodes = append(nodes, n)
	}

	endpoint, _ := decodeString(outs, "control_plane_endpoint")
	inventory, _ := decodeString(outs, "ansible_inventory")

	raw := make(map[string]any, len(outs))
	for k, v := range outs {
		var val any
		if err := json.Unmarshal(v, &val); err == nil {
			raw[k] = val
		}
	}

	return &providers.ProvisionResult{
		Nodes:                nodes,
		ControlPlaneEndpoint: endpoint,
		AnsibleInventory:     inventory,
		RawOutputs:           raw,
	}, nil
}

func decodeString(outs map[string]json.RawMessage, key string) (string, error) {
	raw, ok := outs[key]
	if !ok {
		return "", fmt.Errorf("output %q missing", key)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", err
	}
	return s, nil
}

func decodeStringSlice(outs map[string]json.RawMessage, key string) ([]string, error) {
	raw, ok := outs[key]
	if !ok {
		return nil, fmt.Errorf("output %q missing", key)
	}
	var s []string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	return s, nil
}
