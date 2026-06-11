package orchestrator

import (
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
)

// TestProviderChangeSummary covers the audit-message builder for UpdateProvider
// - the durable record of provider config edits (rename, type, config keys,
// secret-ref). Asserts config VALUES and the secret-ref value never appear (only
// keys / "changed"), so nothing sensitive reaches the audit/SIEM sinks.
func TestProviderChangeSummary(t *testing.T) {
	sp := func(s string) *string { return &s }
	old := db.Provider{Name: "gcp-dev", Type: "gcp", SecretRef: "opord/gcp/dev"}

	tests := []struct {
		name string
		old  db.Provider
		in   ProviderUpdate
		want string
	}{
		{"rename", old, ProviderUpdate{Name: sp("OPORD-GCP")}, `renamed "gcp-dev" to "OPORD-GCP"`},
		{"same name is no-op", old, ProviderUpdate{Name: sp("gcp-dev")}, "no changes"},
		{"type change", db.Provider{Name: "p", Type: "aws"}, ProviderUpdate{Type: sp("gcp")}, "type aws to gcp"},
		{"config keys sorted, values hidden", old, ProviderUpdate{Config: map[string]any{"subnet_ids": "secretval", "region": "eu"}}, "config[region,subnet_ids]"},
		{"secret ref changed shows no value", old, ProviderUpdate{SecretRef: sp("opord/gcp/SENSITIVE")}, "secret_ref changed"},
		{"secret ref unchanged", old, ProviderUpdate{SecretRef: sp("opord/gcp/dev")}, "no changes"},
		{"nothing", old, ProviderUpdate{}, "no changes"},
		{"rename + config combined", old, ProviderUpdate{Name: sp("X"), Config: map[string]any{"region": "z"}}, `renamed "gcp-dev" to "X"; config[region]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := providerChangeSummary(tt.old, tt.in)
			if got != tt.want {
				t.Errorf("providerChangeSummary() = %q, want %q", got, tt.want)
			}
			// Defense in depth: a config value must never leak into the message.
			if tt.name == "config keys sorted, values hidden" && got != "config[region,subnet_ids]" {
				t.Errorf("config value leaked into audit message: %q", got)
			}
		})
	}
}
