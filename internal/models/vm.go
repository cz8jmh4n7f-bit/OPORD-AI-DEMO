package models

// VMSpec is the declarative desired state for a standalone-VM resource
// (kind="vm"). Serialized into resources.spec (jsonb); maps onto the
// modules/vsphere-vm OpenTofu inputs.
type VMSpec struct {
	Template     string   `json:"template"`
	Count        int      `json:"count"`
	NamePrefix   string   `json:"name_prefix"`
	CPU          int      `json:"cpu"`
	MemoryMB     int      `json:"memory_mb"`
	DiskGB       int      `json:"disk_gb"`
	DataDisksGB  []int    `json:"data_disks_gb,omitempty"`
	IPStart      string   `json:"ip_start"`
	Netmask      string   `json:"netmask"`
	Gateway      string   `json:"gateway"`
	DNSServers   []string `json:"dns_servers"`
	DNSSuffix    string   `json:"dns_suffix"`
	SSHUser      string   `json:"ssh_user"`
	SSHPublicKey string   `json:"ssh_public_key"`

	// Cloud / lifecycle knobs (provider-dependent; on-prem ignores some).
	InstanceType string `json:"instance_type,omitempty"` // e.g. t3.micro (AWS)
	Region       string `json:"region,omitempty"`        // AWS region override (else provider's)
	PublicIP     bool   `json:"public_ip,omitempty"`     // assign a public IP (cloud)
	TTLHours     int    `json:"ttl_hours,omitempty"`     // auto-destroy after N hours (0 = never)

	// Deploy target (ADR-0013): a OPORD-managed account to deploy INTO, reusing the
	// provider's own credentials. Provider-neutral - GCP = the target project id
	// (overrides project_id), Azure = the target subscription id (overrides
	// subscription_id); empty = the provider's default. (AWS member accounts need
	// cross-account AssumeRole - a follow-up.)
	TargetAccount string `json:"target_account,omitempty"`
}
