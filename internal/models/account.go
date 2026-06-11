package models

// AccountSpec is the declarative desired state for a provisioned member AWS
// account (kind="account") - the OPORD-orchestrated account factory. One spec
// expands into the layer modules L1-L6 (create_account to baseline to access to 
// secure_vpc to security_baseline to integrations), each a workspace-isolated
// tofu run sequenced by the orchestrator.
type AccountSpec struct {
	// Identity / naming (the former Jenkins parameters).
	CSAID     string `json:"csa_id"`               // unique id; drives account name + resource naming
	ProjectID string `json:"project_id,omitempty"` // human-readable project name
	CloudName string `json:"cloud_name"`           // environment: prod / stage / dev
	Owner     string `json:"owner"`                // primary user (username, no domain)
	Role      string `json:"role,omitempty"`       // Admin / Manager / ReadOnly (access layer)
	Email     string `json:"email"`                // unique root email (alias / catch-all)
	OUID      string `json:"ou_id,omitempty"`      // parent OU; empty = under root

	// Re-run support: when set, skip L1 and operate on this existing account.
	AccountID string `json:"account_id,omitempty"`

	// Networking (L4).
	CreateVPC bool   `json:"create_vpc,omitempty"`
	EnableNAT bool   `json:"enable_nat,omitempty"` // add a NAT gateway to the secure VPC so EKS-into-member node groups get egress (~$32/mo; off = $0 egress-free ZTNA)
	VPCRegion string `json:"vpc_region,omitempty"`
	VPCCidr   string `json:"vpc_cidr,omitempty"` // empty = IPAM allocates a /22 from the Vault pool

	// Access (L3) - SAML path; alternatively vend via the `project` primitive.
	SAMLMetadataVaultPath string `json:"saml_metadata_vault_path,omitempty"`

	// Per-phase skip flags (mirror the reference SKIP_* parameters) for partial
	// re-runs. Empty/false = run the layer.
	Skip AccountSkip `json:"skip,omitempty"`

	// Budget (L2).
	MonthlyBudgetUSD int `json:"monthly_budget_usd,omitempty"` // default 500

	TTLHours int `json:"ttl_hours,omitempty"` // optional auto-decommission (0 = never)

	// Grant-at-creation: vend access to a team AS PART of account creation. Once the
	// factory account/project/subscription reaches ready, OPORD grants the team a role
	// on it (reusing the `project` access primitive - Identity Center on AWS, an Entra
	// group + Azure RBAC, or a GCP project IAM binding) so they can use it without a
	// separate grant step or a console self-grant. Empty GrantTeam = no auto-grant.
	GrantTeam []string `json:"grant_team,omitempty"` // usernames (AWS Identity Center) / UPN-emails (Azure/GCP)
	GrantRole string   `json:"grant_role,omitempty"` // e.g. roles/viewer (GCP), Contributor (Azure), ReadOnlyAccess (AWS)

	// --- Azure-specific (ADR-0009). Ignored by the AWS account factory. ---

	// AzureMode: "adopt" (default - use AzureSubscriptionID) or "create" (provision
	// a new subscription via AzureBillingScopeID). Live test uses adopt-only.
	AzureMode string `json:"azure_mode,omitempty"`
	// AzureSubscriptionID: required when AzureMode=adopt. The GUID of the
	// existing subscription OPORD takes over.
	AzureSubscriptionID string `json:"azure_subscription_id,omitempty"`
	// AzureBillingScopeID: required when AzureMode=create. MCA invoice-section
	// URI (/providers/Microsoft.Billing/billingAccounts/{ba}/billingProfiles/
	// {bp}/invoiceSections/{is}). Provisioning SP needs Invoice Section Owner.
	AzureBillingScopeID string `json:"azure_billing_scope_id,omitempty"`
	// AzureLocation: default Azure region for the base RGs. westeurope by default.
	AzureLocation string `json:"azure_location,omitempty"`
	// AzureAllowedLocations: list passed to the Allowed Locations policy.
	// Empty to ["westeurope", "northeurope"].
	AzureAllowedLocations []string `json:"azure_allowed_locations,omitempty"`
	// AzureVNetCIDR: /22 CIDR for the secure-vnet layer. Empty to IPAM allocates
	// from the opord-azure-vnet-cidr-pools Vault pool.
	AzureVNetCIDR string `json:"azure_vnet_cidr,omitempty"`
	// AzureAllowInboundCIDRs: trusted CIDRs for the NSG allow rules.
	// Empty to ["0.0.0.0/0"] (dev default per ADR-0009).
	AzureAllowInboundCIDRs []string `json:"azure_allow_inbound_cidrs,omitempty"`
	// AzureDefenderPlansStandard: per-workload Defender plans to enable at
	// Standard (paid) tier. Empty to Free-tier CSPM only (V1 default; ~$0/mo).
	AzureDefenderPlansStandard []string `json:"azure_defender_plans_standard,omitempty"`

	// --- GCP project factory (ADR-0011) ---
	// GCPMode signals a GCP account spec (mirrors AzureMode): "create" provisions
	// a new project under the project-factory folder; "adopt" wraps an existing
	// one. Empty to not a GCP spec. The organization id / folder id / billing
	// account come from the provider config (OpenBao), not the spec - so the
	// provisioning identity, not the requester, controls them.
	GCPMode string `json:"gcp_mode,omitempty"`
	// GCPProjectID: for gcp_mode=adopt, the existing project id to wrap.
	GCPProjectID string `json:"gcp_project_id,omitempty"`
	// GCPAllowedLocations: regions allowed by the org-policy layer.
	// Empty to the provider's region (or europe-west1).
	GCPAllowedLocations []string `json:"gcp_allowed_locations,omitempty"`
	// GCPAllowInboundCIDRs: trusted source CIDRs for the ZTNA firewall allow
	// rules. Empty to ["0.0.0.0/0"] (dev default). The VPC CIDR itself reuses
	// VPCCidr (a /22 from IPAM, carved into 3 /24 subnets).
	GCPAllowInboundCIDRs []string `json:"gcp_allow_inbound_cidrs,omitempty"`
}

// AccountSkip toggles individual provisioning layers off for re-runs.
type AccountSkip struct {
	CreateAccount bool `json:"create_account,omitempty"`
	// DeleteDefaultVPCs, when true, skips the setup step that strips the default
	// VPC from every region (mirrors the reference SKIP_DELETE_DEFAULT_VPCS).
	// Default false = the step runs (default VPCs are removed for security).
	DeleteDefaultVPCs bool `json:"delete_default_vpcs,omitempty"`
	Baseline          bool `json:"baseline,omitempty"`
	Access            bool `json:"access,omitempty"`
	SecureVPC         bool `json:"secure_vpc,omitempty"`
	SecurityBaseline  bool `json:"security_baseline,omitempty"`

	// --- Azure-specific layer skips (ADR-0009) ---
	AzureKeyVault          bool `json:"azure_key_vault,omitempty"`          // KV baseline (CMK)
	AzureRBAC              bool `json:"azure_rbac,omitempty"`               // L3 custom roles + Entra groups
	AzureRBACGroups        bool `json:"azure_rbac_groups,omitempty"`        // skip Entra GROUP creation only (role defs still created); use when SP lacks Graph Group.ReadWrite.All / Groups Administrator
	AzureSecureVNet        bool `json:"azure_secure_vnet,omitempty"`        // L4
	AzureSecurityHardening bool `json:"azure_security_hardening,omitempty"` // L5 LAW + activity log
	AzurePolicy            bool `json:"azure_policy,omitempty"`             // policy assignments

	// --- GCP-specific layer skips (ADR-0011) ---
	GCPApis      bool `json:"gcp_apis,omitempty"`       // enable_apis layer (service enablement)
	GCPSecurity  bool `json:"gcp_security,omitempty"`   // KMS keyring/key + CMEK bucket + log sink
	GCPSecureVPC bool `json:"gcp_secure_vpc,omitempty"` // secure VPC + 3 subnets + ZTNA firewall + flow logs
	GCPOrgPolicy bool `json:"gcp_org_policy,omitempty"` // project-level org-policy constraints
	GCPIAM       bool `json:"gcp_iam,omitempty"`        // custom roles + IAM bindings for existing members
}
