// Package vsphere implements providers.Provider for VMware vSphere by wrapping
// the modules/vsphere-k8s OpenTofu module. It only handles Phase 1 (compute +
// network); Kubernetes bootstrap is the provider-agnostic Ansible phase.
package vsphere

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/vcenter"
)

// Config configures the vSphere provider.
type Config struct {
	// ModulesDir is the base directory holding module subdirs (e.g. "./modules").
	ModulesDir string
	// TofuBin is the tofu binary path; empty defaults to "tofu".
	TofuBin string
	// StateConnStr is the pg backend connection string for Tofu state.
	StateConnStr string
	Logger       *slog.Logger
}

// Provider wraps the vSphere OpenTofu module.
type Provider struct {
	cfg       Config
	moduleDir string
	log       *slog.Logger
}

var (
	_ providers.Provider  = (*Provider)(nil)
	_ providers.Inspector = (*Provider)(nil)
)

// New constructs a vSphere Provider.
func New(cfg Config) *Provider {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Provider{
		cfg:       cfg,
		moduleDir: filepath.Join(cfg.ModulesDir, "vsphere-k8s"),
		log:       log,
	}
}

// InspectVMs implements providers.Inspector: it reports live VM state from
// vCenter via the vSphere Web Services API (govmomi). Connection details come
// from req.Config ("url" override or "server") and req.Credentials.
func (p *Provider) InspectVMs(ctx context.Context, req providers.Request) ([]providers.LiveNode, error) {
	endpoint, _ := req.Config["url"].(string)
	if endpoint == "" {
		server, _ := req.Config["server"].(string)
		if server == "" {
			return nil, fmt.Errorf("vsphere: provider config has neither 'url' nor 'server'")
		}
		endpoint = "https://" + server + "/sdk"
	}
	insecure := true
	if v, ok := req.Config["allow_unverified_ssl"].(bool); ok {
		insecure = v
	}

	c, err := vcenter.Connect(ctx, vcenter.Config{
		URL:      endpoint,
		User:     req.Credentials["user"],
		Password: req.Credentials["password"],
		Insecure: insecure,
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = c.Close(ctx) }()

	vms, err := c.ListVMs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]providers.LiveNode, 0, len(vms))
	for _, vm := range vms {
		out = append(out, providers.LiveNode{
			Name:       vm.Name,
			PowerState: vm.PowerState,
			IP:         vm.IP,
			NumCPU:     vm.NumCPU,
			MemoryMB:   vm.MemoryMB,
		})
	}
	return out, nil
}

// Register adds the vSphere provider factory to a registry.
func Register(reg *providers.Registry, cfg Config) {
	reg.Register(models.ProviderVSphere, func() providers.Provider { return New(cfg) })
}

func (p *Provider) Type() models.ProviderType { return models.ProviderVSphere }

// Validate checks a spec against vSphere/Kubernetes constraints.
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
	if strings.TrimSpace(spec.Networking.ControlPlaneEndpoint) == "" {
		errs = append(errs, "networking.control_plane_endpoint is required")
	}
	if strings.TrimSpace(spec.Networking.Gateway) == "" {
		errs = append(errs, "networking.gateway is required")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid vsphere spec: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (p *Provider) runner() *tofu.Runner {
	return tofu.New(p.cfg.TofuBin, p.moduleDir, p.log)
}

func (p *Provider) backendConfig() map[string]string {
	if p.cfg.StateConnStr == "" {
		return nil
	}
	return map[string]string{"conn_str": p.cfg.StateConnStr}
}

// writeVars writes the request's tofu vars to a temp JSON file. The returned
// cleanup func removes the file and is always safe to call.
func (p *Provider) writeVars(req providers.Request) (path string, cleanup func(), err error) {
	noop := func() {}
	data, err := json.Marshal(buildVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling tofu vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-vsphere-*.tfvars.json")
	if err != nil {
		return "", noop, fmt.Errorf("creating vars file: %w", err)
	}
	remove := func() { _ = os.Remove(f.Name()) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		remove()
		return "", noop, fmt.Errorf("writing vars file: %w", err)
	}
	if err := f.Close(); err != nil {
		remove()
		return "", noop, fmt.Errorf("closing vars file: %w", err)
	}
	return f.Name(), remove, nil
}

// prepare runs init + workspace select for an operation.
func (p *Provider) prepare(ctx context.Context, workspace string) (*tofu.Runner, error) {
	r := p.runner()
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, workspace); err != nil {
		return nil, err
	}
	return r, nil
}

// Preflight validates the spec mapping and the Tofu module offline: it writes
// the vars file (proving the ClusterSpec -> Tofu var mapping marshals), runs
// `tofu init -backend=false`, and `tofu validate`. It does NOT contact vCenter
// or the state backend, so it works before a provider lab is reachable.
func (p *Provider) Preflight(ctx context.Context, req providers.Request) (*providers.PreflightResult, error) {
	varsFile, cleanup, err := p.writeVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	_ = varsFile // written to validate marshaling; tofu validate does not consume it

	r := p.runner()
	if err := r.InitBackendless(ctx); err != nil {
		return nil, err
	}
	if err := r.Validate(ctx); err != nil {
		return nil, err
	}

	summary := fmt.Sprintf(
		"module validated; %d control-plane + %d worker node(s), template %q, k8s v%s",
		req.Spec.ControlPlane.Count, req.Spec.Workers.Count, req.Spec.Template, req.Spec.KubernetesVersion,
	)
	return &providers.PreflightResult{Workspace: req.Workspace, ModuleValid: true, Summary: summary}, nil
}

// Plan performs a dry-run.
func (p *Provider) Plan(ctx context.Context, req providers.Request) (*providers.PlanResult, error) {
	r, err := p.prepare(ctx, req.Workspace)
	if err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeVars(req)
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

// Provision reconciles infrastructure to match the spec and returns the nodes.
func (p *Provider) Provision(ctx context.Context, req providers.Request) (*providers.ProvisionResult, error) {
	r, err := p.prepare(ctx, req.Workspace)
	if err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-vsphere-*.tfplan")
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
	return parseOutputs(outs)
}

// Destroy tears down the cluster's infrastructure.
func (p *Provider) Destroy(ctx context.Context, req providers.Request) error {
	r, err := p.prepare(ctx, req.Workspace)
	if err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}
