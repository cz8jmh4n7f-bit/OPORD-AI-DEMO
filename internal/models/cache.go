package models

// CacheSpec is the declarative desired state for a managed in-memory cache
// (kind="cache"). Maps onto modules/aws-elasticache (Redis today).
// Provider-neutral naming so another backend (Valkey, on-prem Redis) could
// implement CacheProvisioner later.
type CacheSpec struct {
	// Name is the replication group id (1-40 chars, lower-case).
	Name string `json:"name"`

	// EngineVersion (Redis). Default "7.1" at the form level.
	EngineVersion string `json:"engine_version,omitempty"`

	// NodeType is the cache instance class (e.g. cache.t4g.micro, cache.r7g.large).
	NodeType string `json:"node_type,omitempty"`

	// NumCacheNodes: 1 = single node (cheapest, no HA); >1 enables Multi-AZ + automatic failover.
	NumCacheNodes int `json:"num_cache_nodes,omitempty"`

	// ParameterGroupName overrides the default per-engine parameter group.
	ParameterGroupName string `json:"parameter_group_name,omitempty"`

	// SubnetIDs is the list of private subnets the cluster spans (at least 2 AZs for HA).
	SubnetIDs []string `json:"subnet_ids,omitempty"`

	// SecurityGroupIDs reuse existing SGs. Empty = module creates a VPC-CIDR-scoped one.
	SecurityGroupIDs []string `json:"security_group_ids,omitempty"`

	// AtRestEncryption enables disk encryption. Default true at the form level.
	AtRestEncryption bool `json:"at_rest_encryption"`

	// InTransitEncryption enables TLS. Required when AuthToken is set. Default true.
	InTransitEncryption bool `json:"in_transit_encryption"`

	// AuthToken is the Redis AUTH password (16-128 chars). Empty = no password.
	// Set out-of-band when possible (e.g. via Secrets Manager + rotation).
	AuthToken string `json:"auth_token,omitempty"`

	// TTLHours: auto-destroy after N hours (0 = never).
	TTLHours int `json:"ttl_hours,omitempty"`

	// TargetAccount: a OPORD-managed account to deploy INTO, reusing the
	// provider's own credentials. Provider-neutral - GCP = the target project id
	// (overrides project_id), Azure = the target subscription id (overrides
	// subscription_id); empty = the provider's default. (AWS member accounts need
	// cross-account AssumeRole - a follow-up.)
	TargetAccount string `json:"target_account,omitempty"`
}
