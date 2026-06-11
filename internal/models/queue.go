package models

// QueueSpec is the declarative desired state for an SQS queue (kind="queue").
// Maps onto modules/aws-sqs. Provider-neutral naming so another backend
// (e.g. NATS/RabbitMQ in the future) could implement QueueProvisioner later.
type QueueSpec struct {
	// Name is the queue base name. For FIFO, the module appends ".fifo".
	Name string `json:"name"`

	// FIFO selects FIFO ordering + exactly-once. Standard otherwise (higher throughput).
	FIFO bool `json:"fifo,omitempty"`

	// VisibilityTimeoutSeconds is how long a received message is invisible to others.
	// Set to the longest consumer processing time + buffer. Default 30.
	VisibilityTimeoutSeconds int `json:"visibility_timeout_seconds,omitempty"`

	// MessageRetentionSeconds is how long unconsumed messages survive.
	// Default 345600 (4 days). Range 60-1209600 (14 days).
	MessageRetentionSeconds int `json:"message_retention_seconds,omitempty"`

	// MaxMessageSizeBytes caps per-message size. Default 262144 (256 KB, max).
	MaxMessageSizeBytes int `json:"max_message_size_bytes,omitempty"`

	// ReceiveWaitTimeSeconds enables long-polling (0-20). Set > 0 to cut empty-receive cost.
	ReceiveWaitTimeSeconds int `json:"receive_wait_time_seconds,omitempty"`

	// DLQEnabled auto-creates a sibling dead-letter queue + redrive policy.
	DLQEnabled bool `json:"dlq_enabled,omitempty"`

	// DLQMaxReceiveCount is the failures-before-DLQ threshold. Default 5.
	DLQMaxReceiveCount int `json:"dlq_max_receive_count,omitempty"`

	// KMSKeyARN selects customer-managed KMS encryption. Empty = AWS-managed SQS (free).
	KMSKeyARN string `json:"kms_key_arn,omitempty"`

	// TTLHours: auto-destroy after N hours (0 = never).
	TTLHours int `json:"ttl_hours,omitempty"`

	// Deploy target (ADR-0013): a OPORD-managed account to deploy INTO, reusing the
	// provider's own credentials. Provider-neutral - GCP = the target project id
	// (overrides project_id), Azure = the target subscription id (overrides
	// subscription_id); empty = the provider's default.
	TargetAccount string `json:"target_account,omitempty"`
}
