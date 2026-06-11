package models

// S3Spec is the declarative desired state for an S3 bucket (kind="s3").
// Maps onto modules/aws-s3-bucket. Provider-neutral naming so another backend
// (e.g. MinIO) could implement S3Provisioner later.
type S3Spec struct {
	// Name is the globally-unique S3 bucket name (3-63 chars, AWS naming rules).
	Name string `json:"name"`

	// Versioning enables object versioning (recommended for any non-throwaway bucket).
	// Default true at the form level.
	Versioning bool `json:"versioning"`

	// BlockPublicAccess sets the 4 BPA flags. Default true at the form level;
	// opt-out only for explicit public-website cases.
	BlockPublicAccess bool `json:"block_public_access"`

	// KMSKeyARN selects SSE-KMS. Empty = SSE-S3 (AES256, free). Setting a CMK
	// adds $1/key/month + per-request KMS charges.
	KMSKeyARN string `json:"kms_key_arn,omitempty"`

	// LifecycleGlacierDays moves objects to Glacier Deep Archive after N days.
	// 0 = no lifecycle rule.
	LifecycleGlacierDays int `json:"lifecycle_glacier_days,omitempty"`

	// TTLHours: auto-destroy the bucket after N hours (0 = never). Useful for
	// lab/POC buckets. The TTL reaper only acts on empty buckets.
	TTLHours int `json:"ttl_hours,omitempty"`

	// Deploy target (ADR-0013): a OPORD-managed account to deploy INTO, reusing the
	// provider's own credentials. Provider-neutral - GCP = the target project id
	// (overrides project_id), Azure = the target subscription id (overrides
	// subscription_id); empty = the provider's default. (AWS member accounts need
	// cross-account AssumeRole - a follow-up.)
	TargetAccount string `json:"target_account,omitempty"`
}
