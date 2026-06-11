package tofu

import (
	"strings"
	"testing"
)

func TestStateEncryption(t *testing.T) {
	t.Cleanup(func() { SetStateEncryptionPassphrase("") }) // don't leak global state

	// Disabled by default / on an empty passphrase (backward compatible).
	SetStateEncryptionPassphrase("")
	if StateEncryptionEnabled() {
		t.Fatal("expected disabled on empty passphrase")
	}
	if stateEncryptionEnv() != "" {
		t.Fatalf("expected empty env when disabled, got %q", stateEncryptionEnv())
	}

	// Enabled: the env carries a well-formed TF_ENCRYPTION with our pbkdf2/aes_gcm
	// config and the migration-safe enforced=false.
	SetStateEncryptionPassphrase("a-test-passphrase-32-chars-long!")
	if !StateEncryptionEnabled() {
		t.Fatal("expected enabled after setting a passphrase")
	}
	env := stateEncryptionEnv()
	if !strings.HasPrefix(env, "TF_ENCRYPTION=") {
		t.Fatalf("expected a TF_ENCRYPTION env entry, got %q", env)
	}
	for _, want := range []string{
		`key_provider "pbkdf2" "opord"`,
		`method "aes_gcm" "opord"`,
		"enforced = false",
		`passphrase = "a-test-passphrase-32-chars-long!"`,
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("TF_ENCRYPTION missing %q\n---\n%s", want, env)
		}
	}
}
