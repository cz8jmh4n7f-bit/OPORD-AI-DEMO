package models

// TableSpec is the declarative desired state for a managed NoSQL table
// (kind="table"). Maps onto modules/aws-dynamodb (AWS DynamoDB). Provider-neutral
// naming so another backend could implement TableProvisioner later.
type TableSpec struct {
	Name         string `json:"name"`          // table name
	HashKey      string `json:"hash_key"`      // partition key attribute name
	HashKeyType  string `json:"hash_key_type"` // S | N | B (string/number/binary); default S
	RangeKey     string `json:"range_key,omitempty"`
	RangeKeyType string `json:"range_key_type,omitempty"` // S | N | B; required if RangeKey set
	// BillingMode: PAY_PER_REQUEST (on-demand, default - cheapest/simplest) or
	// PROVISIONED (then ReadCapacity/WriteCapacity apply).
	BillingMode   string `json:"billing_mode,omitempty"`
	ReadCapacity  int    `json:"read_capacity,omitempty"`
	WriteCapacity int    `json:"write_capacity,omitempty"`
	TTLHours      int    `json:"ttl_hours,omitempty"` // auto-destroy after N hours (0 = never)

	// Deploy target (ADR-0013): a OPORD-managed account to deploy INTO, reusing the
	// provider's own credentials. Provider-neutral - GCP = the target project id
	// (overrides project_id), Azure = the target subscription id (overrides
	// subscription_id); empty = the provider's default. (AWS member accounts need
	// cross-account AssumeRole - a follow-up.)
	TargetAccount string `json:"target_account,omitempty"`
}
