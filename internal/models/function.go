package models

// FunctionSpec is the declarative desired state for a serverless function
// (kind="function"). Maps onto modules/aws-lambda (AWS Lambda). Provider-neutral
// naming so another backend (GCP Cloud Functions, ...) could implement
// FunctionProvisioner later.
type FunctionSpec struct {
	Runtime    string `json:"runtime"`     // e.g. python3.12, nodejs20.x; default python3.12
	Handler    string `json:"handler"`     // entry point, e.g. index.handler; default index.handler
	MemoryMB   int    `json:"memory_mb"`   // default 128
	TimeoutSec int    `json:"timeout_sec"` // default 10
	// Region/location the function deploys to. Empty = the provider's configured
	// region. AWS Lambda ignores it (region comes from the provider/STS env); GCP
	// Cloud Functions honors it (else a function lands in the provider's default
	// region regardless of the form selection).
	Region string `json:"region,omitempty"`
	// Code source: an S3 zip (bucket+key). When empty, OPORD deploys a minimal
	// built-in "hello" handler (python) so the function is immediately invokable.
	S3Bucket string            `json:"s3_bucket,omitempty"`
	S3Key    string            `json:"s3_key,omitempty"`
	EnvVars  map[string]string `json:"env_vars,omitempty"`
	TTLHours int               `json:"ttl_hours,omitempty"` // auto-destroy after N hours (0 = never)

	// Deploy target (ADR-0013): a OPORD-managed account to deploy INTO, reusing the
	// provider's own credentials. Provider-neutral - GCP = the target project id
	// (overrides project_id), Azure = the target subscription id (overrides
	// subscription_id); empty = the provider's default.
	TargetAccount string `json:"target_account,omitempty"`
}
