package models

// DatabaseSpec is the declarative desired state for a managed-database resource
// (kind="database"). Maps onto modules/aws-rds (AWS RDS). The master password is
// never handled by OPORD - RDS manages it in Secrets Manager.
type DatabaseSpec struct {
	Engine        string `json:"engine"`         // postgres | mysql
	Version       string `json:"version"`        // engine version, e.g. "16"
	InstanceClass string `json:"instance_class"` // e.g. db.t3.micro
	StorageGB     int    `json:"storage_gb"`
	DBName        string `json:"db_name"`
	Username      string `json:"username"`
	MultiAZ       bool   `json:"multi_az,omitempty"`
	PublicAccess  bool   `json:"public_access,omitempty"`
	TTLHours      int    `json:"ttl_hours,omitempty"` // auto-destroy after N hours (0 = never)

	// Deploy target (ADR-0013): a OPORD-managed account to deploy INTO, reusing the
	// provider's own credentials. Provider-neutral - GCP = the target project id
	// (overrides project_id), Azure = the target subscription id (overrides
	// subscription_id); empty = the provider's default. (AWS member accounts need
	// cross-account AssumeRole - a follow-up.)
	TargetAccount string `json:"target_account,omitempty"`

	// Passwordless auth (opt-in). "" / "password" (default) = a generated password
	// stored in OpenBao. "iam" = GCP Cloud SQL IAM database authentication - no
	// static password; the principal connects with a short-lived IAM token.
	AuthMode      string `json:"auth_mode,omitempty"`
	AuthPrincipal string `json:"auth_principal,omitempty"` // IAM user / service-account email (auth_mode=iam)
}
