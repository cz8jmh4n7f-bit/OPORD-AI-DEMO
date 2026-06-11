package models

// StackSpec is the declarative desired state for a generic "stack" resource: an
// arbitrary OpenTofu root module run by OPORD. This is the "anything" escape
// hatch - the module can create any resource the provider supports (any AWS
// resource, etc.). The module must NOT declare its own backend (OPORD injects a
// workspace-isolated pg backend) and gets its variables from OPORD.
type StackSpec struct {
	ModuleDir string         `json:"module_dir"` // path to a root Tofu module (no backend block)
	Variables map[string]any `json:"variables,omitempty"`
	TTLHours  int            `json:"ttl_hours,omitempty"` // auto-destroy after N hours (0 = never)

	// Deploy target (ADR-0013): a OPORD-managed account to deploy INTO, reusing the
	// provider's own credentials. Provider-neutral - GCP = the target project id
	// (overrides project_id), Azure = the target subscription id (overrides
	// subscription_id); empty = the provider's default. (AWS member accounts need
	// cross-account AssumeRole - a follow-up.)
	TargetAccount string `json:"target_account,omitempty"`
}
