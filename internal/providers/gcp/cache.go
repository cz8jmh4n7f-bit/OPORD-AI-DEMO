package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// opordVPCRe matches the account-factory's VPC name (opord-<csa>-vpc).
var opordVPCRe = regexp.MustCompile(`^opord-.*-vpc$`)

// factoryNetwork returns the self-link of a governed project's factory VPC
// (opord-*-vpc) for Memorystore's authorized_network - a factory project has no
// "default" network, so Memorystore must be told which VPC to attach to. Best-effort
// over the Compute REST API with the resolved OAuth token; returns "" when it can't
// determine one (the module then leaves authorized_network unset = the default network).
func factoryNetwork(ctx context.Context, creds map[string]string, project string) string {
	token := creds["access_token"]
	if token == "" {
		return ""
	}
	url := fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/%s/global/networks", project)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var out struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ""
	}
	for _, n := range out.Items {
		if opordVPCRe.MatchString(n.Name) {
			return fmt.Sprintf("projects/%s/global/networks/%s", project, n.Name)
		}
	}
	return ""
}

// factorySubnet returns the partial self-link of a subnet that BELONGS TO the
// given factory VPC (matched by the subnet's network field, not its name) plus
// the subnet's region. GKE needs an explicit subnetwork - and a cluster region
// that matches it - when deploying into a governed project that has no "default"
// network. Searching by VPC membership across ALL regions (aggregatedList) avoids
// two traps: (1) a name regex like opord-*-subnet also matches an unrelated VM's
// own subnet (opord-<vm>-subnet) in a different VPC; (2) the factory VPC's subnets
// live in the account's region (e.g. europe-west1), not the provider's default
// region. Best-effort; ("","") when it can't determine one.
func factorySubnet(ctx context.Context, creds map[string]string, project, vpcName string) (subnet, region string) {
	token := creds["access_token"]
	if token == "" || project == "" || vpcName == "" {
		return "", ""
	}
	url := fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/%s/aggregated/subnetworks", project)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", ""
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	var out struct {
		Items map[string]struct {
			Subnetworks []struct {
				Name    string `json:"name"`
				Network string `json:"network"`
				Region  string `json:"region"`
			} `json:"subnetworks"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", ""
	}
	suffix := "/networks/" + vpcName
	for _, item := range out.Items {
		for _, s := range item.Subnetworks {
			if strings.HasSuffix(s.Network, suffix) {
				reg := s.Region
				if i := strings.LastIndex(reg, "/"); i >= 0 {
					reg = reg[i+1:]
				}
				return fmt.Sprintf("projects/%s/regions/%s/subnetworks/%s", project, reg, s.Name), reg
			}
		}
	}
	return "", ""
}

// withTargetNetwork adds authorized_network (the factory VPC) to the config when
// deploying Memorystore into a governed project (target_account set). No-op otherwise.
func withTargetNetwork(ctx context.Context, cfg map[string]any, creds map[string]string, targetAccount string) map[string]any {
	if targetAccount == "" {
		return cfg
	}
	net := factoryNetwork(ctx, creds, targetAccount)
	if net == "" {
		return cfg
	}
	out := make(map[string]any, len(cfg)+1)
	for k, v := range cfg {
		out[k] = v
	}
	out["authorized_network"] = net
	return out
}

// CacheProvisioner: a Memorystore for Redis instance via modules/gcp-memorystore.
// The auth token / access key is intentionally NOT returned (OPORD never holds it).

var _ providers.CacheProvisioner = (*Provider)(nil)

func (p *Provider) writeCacheVars(req providers.CacheRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildCacheVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling gcp cache vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-gcp-redis-*.tfvars.json")
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

func (p *Provider) PreflightCache(ctx context.Context, req providers.CacheRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	_, cleanup, err := p.writeCacheVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.redisModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionCache(ctx context.Context, req providers.CacheRequest) (*providers.CacheResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	req.Config = withTargetNetwork(ctx, req.Config, req.Credentials, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.redisModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeCacheVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-gcp-redis-*.tfplan")
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
	return &providers.CacheResult{
		PrimaryEndpoint: outString(outs, "primary_endpoint"),
		ReaderEndpoint:  outString(outs, "reader_endpoint"),
		Port:            outInt(outs, "port"),
		ID:              outString(outs, "id"),
		RawOutputs:      rawMap(outs),
	}, nil
}

func (p *Provider) DestroyCache(ctx context.Context, req providers.CacheRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	req.Config = withTargetNetwork(ctx, req.Config, req.Credentials, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.redisModuleDir, p.log)
	r.SetEnv(gcpTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeCacheVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildCacheVars maps a CacheRequest onto the modules/gcp-memorystore inputs.
func buildCacheVars(req providers.CacheRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config
	name := spec.Name
	if name == "" {
		name = req.Name
	}
	if name == "" {
		name = "opord-" + req.Workspace
	}
	region := cfgStringDefault(cfg, "region", "europe-west1")

	vars := map[string]any{
		"name":               safeName(name, 40),
		"region":             region,
		"replicated":         spec.NumCacheNodes > 1,
		"transit_encryption": spec.InTransitEncryption,
		"labels": map[string]string{
			"opord_kind":      "cache",
			"opord_workspace": safeName(req.Workspace, 60),
		},
	}
	if v := redisVersion(spec.EngineVersion); v != "" {
		vars["redis_version"] = v
	}
	if an := cfgStringDefault(cfg, "authorized_network", ""); an != "" {
		vars["authorized_network"] = an
	}
	return vars
}

// redisVersion maps a loose engine version ("7.0", "6.x") onto a Memorystore
// REDIS_X_Y string; empty falls back to the module default.
func redisVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToUpper(v), "REDIS_") {
		return strings.ToUpper(v)
	}
	major := v
	if i := strings.IndexAny(v, ".x"); i > 0 {
		major = v[:i]
	}
	switch major {
	case "7":
		return "REDIS_7_0"
	case "6":
		return "REDIS_6_X"
	case "5":
		return "REDIS_5_0"
	}
	return ""
}

// outInt extracts a single number-valued tofu output as an int.
func outInt(outs map[string]json.RawMessage, key string) int {
	raw, ok := outs[key]
	if !ok {
		return 0
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return int(n)
	}
	return 0
}
