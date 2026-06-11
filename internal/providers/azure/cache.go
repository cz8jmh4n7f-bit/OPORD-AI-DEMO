package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/tofu"
)

// CacheProvisioner: Azure Cache for Redis via modules/azure-rediscache. The
// provider-neutral CacheSpec ("managed in-memory cache") maps onto an Azure
// Redis cache - so the existing first-class /caches surface works for Azure
// too. AWS-specific fields (NodeType cache.* classes, SubnetIDs, AuthToken,
// at-rest encryption - a Premium-only feature) have no Basic/Standard Azure
// equivalent and are ignored; NumCacheNodes>1 selects the replicated Standard
// SKU. The access keys are never persisted (TLS-only; read them from Azure).

var _ providers.CacheProvisioner = (*Provider)(nil)

func (p *Provider) redisModuleDir() string {
	return p.cfg.ModulesDir + "/azure-rediscache"
}

func (p *Provider) writeRedisVars(req providers.CacheRequest) (string, func(), error) {
	noop := func() {}
	data, err := json.Marshal(buildRedisVars(req))
	if err != nil {
		return "", noop, fmt.Errorf("marshaling azure redis vars: %w", err)
	}
	f, err := os.CreateTemp("", "opord-azure-redis-*.tfvars.json")
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
	_, cleanup, err := p.writeRedisVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	r := tofu.New(p.cfg.TofuBin, p.redisModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.InitBackendless(ctx); err != nil {
		return err
	}
	return r.Validate(ctx)
}

func (p *Provider) ProvisionCache(ctx context.Context, req providers.CacheRequest) (*providers.CacheResult, error) {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.redisModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return nil, err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return nil, err
	}
	varsFile, cleanup, err := p.writeRedisVars(req)
	if err != nil {
		return nil, err
	}
	defer cleanup()

	planFile, err := os.CreateTemp("", "opord-azure-redis-*.tfplan")
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
		PrimaryEndpoint: azureOutString(outs, "primary_endpoint"),
		Port:            azureOutInt(outs, "ssl_port"),
		ID:              azureOutString(outs, "cache_name"),
		RawOutputs:      rawMap(outs),
	}, nil
}

func (p *Provider) DestroyCache(ctx context.Context, req providers.CacheRequest) error {
	req.Config = targetCfg(req.Config, req.Spec.TargetAccount)
	r := tofu.New(p.cfg.TofuBin, p.redisModuleDir(), p.log)
	r.SetEnv(azureTofuEnv(req.Credentials, req.Config, ""))
	if err := r.Init(ctx, p.backendConfig()); err != nil {
		return err
	}
	if err := r.SelectWorkspace(ctx, req.Workspace); err != nil {
		return err
	}
	varsFile, cleanup, err := p.writeRedisVars(req)
	if err != nil {
		return err
	}
	defer cleanup()
	return r.Destroy(ctx, varsFile)
}

// buildRedisVars maps the provider-neutral CacheSpec onto modules/azure-rediscache.
func buildRedisVars(req providers.CacheRequest) map[string]any {
	spec := req.Spec
	cfg := req.Config

	location := cfgString(cfg, "location")
	if location == "" {
		location = "westeurope"
	}

	namePrefix := spec.Name
	if namePrefix == "" {
		namePrefix = req.Name
	}
	if namePrefix == "" {
		namePrefix = "opord-" + safePrefix(req.Workspace, 12)
	} else {
		namePrefix = safePrefix(namePrefix, 24)
	}

	// NumCacheNodes>1 to the replicated Standard SKU; else Basic (single node).
	sku := "Basic"
	if spec.NumCacheNodes > 1 {
		sku = "Standard"
	}

	vars := map[string]any{
		"location":    location,
		"name_prefix": namePrefix,
		"environment": cfgStringDefault(cfg, "environment", "dev"),
		"sku_name":    sku,
		"family":      "C",
		"capacity":    0, // 250MB - cheapest; the spec's AWS node_type has no direct map
	}
	if spec.EngineVersion != "" {
		// Azure wants a major version ("6"); take the leading integer of e.g. "7.1".
		if v := majorVersion(spec.EngineVersion); v != "" {
			vars["redis_version"] = v
		}
	}
	return vars
}

// majorVersion returns the leading integer portion of a version string ("7.1" to "7").
func majorVersion(v string) string {
	for i := 0; i < len(v); i++ {
		if v[i] < '0' || v[i] > '9' {
			return v[:i]
		}
	}
	return v
}
