package aws

import (
	"context"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// CacheProvisioner: managed Redis via modules/aws-elasticache (ElastiCache
// replication group). Needs subnet_ids (private subnets) - from the spec or
// the provider config - like EKS. Uniform tofu flow - see module.go.

var _ providers.CacheProvisioner = (*Provider)(nil)

const cachePrefix = "opord-aws-cache"

// PreflightCache validates the var mapping + the aws-elasticache module offline.
func (p *Provider) PreflightCache(ctx context.Context, req providers.CacheRequest) error {
	return p.preflightModule(ctx, p.cacheModuleDir, cachePrefix, req.Credentials, req.Config, buildCacheVars(req))
}

// ProvisionCache creates the cache (tofu apply) for the workspace.
func (p *Provider) ProvisionCache(ctx context.Context, req providers.CacheRequest) (*providers.CacheResult, error) {
	outs, err := p.applyModule(ctx, p.cacheModuleDir, cachePrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildCacheVars(req))
	if err != nil {
		return nil, err
	}
	return &providers.CacheResult{
		PrimaryEndpoint: dbOutString(outs, "primary_endpoint_address"),
		ReaderEndpoint:  dbOutString(outs, "reader_endpoint_address"),
		Port:            dbOutInt(outs, "port"),
		ID:              dbOutString(outs, "replication_group_id"),
		RawOutputs:      rawMap(outs),
	}, nil
}

// DestroyCache tears down the cache for the request's workspace.
func (p *Provider) DestroyCache(ctx context.Context, req providers.CacheRequest) error {
	return p.destroyModule(ctx, p.cacheModuleDir, cachePrefix, req.Workspace, req.Credentials, req.Config, req.Spec.TargetAccount, buildCacheVars(req))
}

// buildCacheVars maps a CacheRequest onto the modules/aws-elasticache inputs.
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
	vars := map[string]any{
		"region":                cfgString(cfg, "region"),
		"name":                  name,
		"at_rest_encryption":    spec.AtRestEncryption,
		"in_transit_encryption": spec.InTransitEncryption,
		"auth_token":            spec.AuthToken,
	}
	if spec.EngineVersion != "" {
		vars["engine_version"] = spec.EngineVersion
	}
	if spec.NodeType != "" {
		vars["node_type"] = spec.NodeType
	}
	if spec.NumCacheNodes > 0 {
		vars["num_cache_nodes"] = spec.NumCacheNodes
	}
	if spec.ParameterGroupName != "" {
		vars["parameter_group_name"] = spec.ParameterGroupName
	}
	// subnet_ids: spec wins, else the provider config (like EKS).
	subnets := spec.SubnetIDs
	if len(subnets) == 0 {
		subnets = cfgStringSlice(cfg, "subnet_ids")
	}
	if len(subnets) > 0 {
		vars["subnet_ids"] = subnets
	}
	if len(spec.SecurityGroupIDs) > 0 {
		vars["security_group_ids"] = spec.SecurityGroupIDs
	}
	return vars
}
