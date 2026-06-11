package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// VMProvisioner: Compute Engine VM via modules/gcp-vm (google_compute_instance
// in an auto VPC/subnet with a locked firewall and optional external IP).

func (p *Provider) writeVMVars(req providers.VMRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildVMVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gcp vm vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-vm-*.tfvars.json")
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

// PreflightVM validates the gcp-vm module + var mapping offline (no API calls).
func (p *Provider) PreflightVM(ctx context.Context, req providers.VMRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeVMVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, req.Spec.Region))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

// ProvisionVM creates the VM(s) (tofu apply) for the request's workspace.
func (p *Provider) ProvisionVM(ctx context.Context, req providers.VMRequest) (*providers.VMResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.vmModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, req.Spec.Region))
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

	planFile, err := os.CreateTemp("", "opord-gcp-vm-*.tfplan")
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
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, req.Spec.Region))
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

// buildVMVars maps a VMRequest onto the modules/gcp-vm inputs. Default image:
// Ubuntu 22.04 LTS; default machine type: e2-micro (free-tier-ish).
func buildVMVars(req providers.VMRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config

	region := spec.Region
	if region == "" {
		region = cfgString(cfg, "region")
	}
	if region == "" {
		region = "europe-west1"
	}
	// The zone must lie inside the resolved region. The provider config's zone is
	// for the provider's DEFAULT region, so when a VM overrides the region (e.g. a
	// deploy into a region-restricted managed project) the cfg zone would put the
	// instance in a different region than its subnet - GCP rejects that as "scope
	// of the specified subnetwork doesn't match the scope of the instance". Use the
	// cfg zone only when it belongs to the resolved region; otherwise derive one.
	zone := cfgString(cfg, "zone")
	if zone == "" || !strings.HasPrefix(zone, region+"-") {
		zone = region + "-b"
	}

	namePrefix := spec.NamePrefix
	if namePrefix == "" {
		namePrefix = "opord-" + safeName(req.Workspace, 12)
	} else {
		namePrefix = safeName(namePrefix, 30)
	}

	count := spec.Count
	if count <= 0 {
		count = 1
	}

	machine := spec.InstanceType
	if machine == "" {
		machine = "e2-micro"
	}

	image := spec.Template
	if image == "" {
		image = "ubuntu-os-cloud/ubuntu-2204-lts"
	}

	sshUser := spec.SSHUser
	if sshUser == "" {
		sshUser = "opord"
	}

	diskGB := spec.DiskGB
	if diskGB <= 0 {
		diskGB = 10
	}

	publicIP := spec.PublicIP
	if gcpIsProd(cfg) {
		publicIP = false
	}
	sshSourcesDefault := []string{"0.0.0.0/0"}
	if gcpIsProd(cfg) {
		sshSourcesDefault = []string{"10.0.0.0/8"}
	}

	return map[string]any{
		"region":         region,
		"zone":           zone,
		"name_prefix":    namePrefix,
		"environment":    cfgStringDefault(cfg, "environment", "dev"),
		"vm_count":       count,
		"machine_type":   machine,
		"image":          image,
		"ssh_user":       sshUser,
		"ssh_public_key": spec.SSHPublicKey,
		"disk_gb":        diskGB,
		"public_ip":      publicIP,
		"allow_ssh_from": cfgStringListDefault(cfg, "gcp_allow_ssh_from", sshSourcesDefault),
	}
}

// safeName trims a string to maxLen and removes characters rejected by GCP
// resource naming (lowercase letters, digits, hyphens; must start with a letter).
func safeName(s string, maxLen int) string {
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
			out = append(out, c+32)
		}
	}
	if len(out) == 0 || (out[0] >= '0' && out[0] <= '9') {
		out = append([]byte("f"), out...)
	}
	return string(out)
}

// outStrings unmarshals a tofu list-of-strings output.
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

// rawMap returns every output as a generic any.
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
