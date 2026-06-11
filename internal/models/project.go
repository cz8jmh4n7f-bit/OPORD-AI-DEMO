package models

// ProjectSpec is the declarative desired state for an access-vending "project"
// (kind="project"). Provider-neutral: on AWS it maps onto modules/aws-sso-project
// (an IAM Identity Center group bound to a permission set on an account); on Azure
// onto modules/azure-access (an Entra security group bound to an Azure RBAC role
// at a subscription / resource-group scope). In both cases a project = one group
// + a role-binding + the listed users as members; day-2 "add/remove member" is a
// re-apply with an updated UserNames list (idempotent).
type ProjectSpec struct {
	AccountID string   `json:"account_id"` // AWS: existing 12-digit account the group is granted access to
	UserNames []string `json:"user_names"` // existing usernames (AWS Identity Center) / UPNs-emails (Entra) added to the group

	// --- AWS (IAM Identity Center) ---
	// Permission set: create a new one (PermissionSetName + ManagedPolicyARNs),
	// OR reference an existing one (ExistingPermissionSetARN).
	PermissionSetName        string   `json:"permission_set_name,omitempty"`
	ManagedPolicyARNs        []string `json:"managed_policy_arns,omitempty"`
	SessionDuration          string   `json:"session_duration,omitempty"` // ISO-8601, e.g. PT8H
	ExistingPermissionSetARN string   `json:"existing_permission_set_arn,omitempty"`

	// Identity Center instance: auto-derived from the org's single instance when empty.
	SSOInstanceARN  string `json:"sso_instance_arn,omitempty"`
	IdentityStoreID string `json:"identity_store_id,omitempty"`

	// --- Azure (Entra group + Azure RBAC) ---
	// SubscriptionID is the target subscription whose scope the role is granted at
	// (the Azure analog of AccountID). ResourceGroup, when set, narrows the scope
	// from the whole subscription to that resource group. RoleName is the built-in
	// (or custom) Azure RBAC role assigned to the group (e.g. Reader, Contributor).
	SubscriptionID string `json:"subscription_id,omitempty"`
	ResourceGroup  string `json:"resource_group,omitempty"`
	RoleName       string `json:"role_name,omitempty"`

	// PIMEligible makes the group *eligible* for the role via Privileged Identity
	// Management (just-in-time: members activate the role on demand) instead of a
	// permanent assignment. Requires Microsoft Entra ID P2 on the tenant.
	PIMEligible bool `json:"pim_eligible,omitempty"`

	// --- GCP (Workforce Identity Federation: Entra -> GCP access) ---
	// When EntraGroupIDs is set, RoleName is granted on the project (AccountID, or
	// the provider's project_id) to those federated Entra groups via a WIF
	// principalSet (principalSet://.../workforcePools/<pool>/group/<id>), instead of
	// - or alongside - the bare UserNames. WorkforcePoolID defaults to the provider
	// config's workforce_pool_id (set up once by modules/gcp-workforce-pool).
	WorkforcePoolID string   `json:"workforce_pool_id,omitempty"`
	EntraGroupIDs   []string `json:"entra_group_ids,omitempty"`

	// --- Common ---
	GroupPrefix string `json:"group_prefix,omitempty"` // default "opord-"
	TTLHours    int    `json:"ttl_hours,omitempty"`    // auto-revoke access after N hours (0 = never)
}
