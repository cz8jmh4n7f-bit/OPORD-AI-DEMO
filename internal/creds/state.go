package creds

import (
	"context"
	"os"
)

// StateEncryptionPassphrase returns the OpenTofu state-encryption passphrase:
// OPORD_STATE_ENCRYPTION_PASSPHRASE if set, else the secrets store at
// opord/state-encryption (key "passphrase"). Empty result = encryption disabled.
// OpenBao is the source of truth for the passphrase; the env var is a convenience /
// override for setups without a secret store.
func StateEncryptionPassphrase(ctx context.Context, r Resolver) string {
	if p := os.Getenv("OPORD_STATE_ENCRYPTION_PASSPHRASE"); p != "" {
		return p
	}
	if r == nil {
		return ""
	}
	m, err := r.ReadSecret(ctx, "opord/state-encryption")
	if err != nil {
		return ""
	}
	return m["passphrase"]
}
