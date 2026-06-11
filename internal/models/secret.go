package models

// SecretSpec is the declarative desired state for a Secrets Manager secret
// (kind="secret"). Maps onto modules/aws-secret. The secret VALUE is set
// out-of-band (CLI/console/Vault-sync) - OPORD never holds plaintext credentials.
type SecretSpec struct {
	// Name is the path-like secret identifier (e.g. "prod/api/jwt-key").
	Name string `json:"name"`

	// Description appears in the AWS console + audit (for ops).
	Description string `json:"description,omitempty"`

	// KMSKeyARN selects a customer-managed KMS key. Empty = aws/secretsmanager
	// (AWS-managed, free).
	KMSKeyARN string `json:"kms_key_arn,omitempty"`

	// RecoveryWindowDays is the soft-delete grace period (7-30). 0 = force-delete
	// (no recovery; only safe in dev/lab).
	RecoveryWindowDays int `json:"recovery_window_days,omitempty"`

	// RotationLambdaARN enables automatic rotation by the named Lambda.
	// Empty = no rotation.
	RotationLambdaARN string `json:"rotation_lambda_arn,omitempty"`

	// RotationDays is the rotation interval (ignored without a rotation Lambda).
	RotationDays int `json:"rotation_days,omitempty"`

	// TTLHours: auto-destroy after N hours (0 = never).
	TTLHours int `json:"ttl_hours,omitempty"`

	// TargetAccount (ADR-0013): a OPORD-managed account to deploy INTO, reusing
	// the provider's own credentials. Provider-neutral - GCP = target project id
	// (overrides project_id), Azure = target subscription id (overrides
	// subscription_id); empty = the provider's default.
	TargetAccount string `json:"target_account,omitempty"`
}
