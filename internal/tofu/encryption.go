package tofu

import (
	"fmt"
	"sync"
)

// OpenTofu native state encryption, configured once at startup and applied to every
// tofu run via the TF_ENCRYPTION environment variable (so no module HCL changes).
// When a passphrase is set, the pg-backend state (and plan files) are encrypted at
// rest with AES-GCM, the key derived from the passphrase via pbkdf2.
//
// enforced = false: existing UNENCRYPTED state is still read, and the NEXT write
// encrypts it - a safe, migration-friendly rollout. Empty passphrase = disabled
// (default; fully backward compatible). Once enabled, the passphrase MUST persist -
// state written while it was set can only be read back with the same passphrase.

var (
	encMu     sync.RWMutex
	encConfig string // the TF_ENCRYPTION body, empty = encryption disabled
)

// SetStateEncryptionPassphrase configures OpenTofu state encryption for all runners.
// An empty passphrase disables it. OpenTofu's pbkdf2 requires the passphrase to be at
// least 16 bytes. Call once at startup, before any tofu run.
func SetStateEncryptionPassphrase(passphrase string) {
	encMu.Lock()
	defer encMu.Unlock()
	if passphrase == "" {
		encConfig = ""
		return
	}
	encConfig = fmt.Sprintf(`key_provider "pbkdf2" "opord" {
  passphrase = %q
}
method "aes_gcm" "opord" {
  keys = key_provider.pbkdf2.opord
}
state {
  method   = method.aes_gcm.opord
  enforced = false
}
plan {
  method = method.aes_gcm.opord
}`, passphrase)
}

// StateEncryptionEnabled reports whether a passphrase is configured.
func StateEncryptionEnabled() bool {
	encMu.RLock()
	defer encMu.RUnlock()
	return encConfig != ""
}

// stateEncryptionEnv returns the "TF_ENCRYPTION=..." env entry, or "" when disabled.
func stateEncryptionEnv() string {
	encMu.RLock()
	defer encMu.RUnlock()
	if encConfig == "" {
		return ""
	}
	return "TF_ENCRYPTION=" + encConfig
}
