// Package creds resolves provider credentials. The environment-backed resolver
// is the MVP; a Vault-backed resolver (reading provider.SecretRef) will satisfy
// the same shape later.
package creds

import (
	"context"
	"os"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
)

// EnvResolver reads credentials from environment variables.
type EnvResolver struct{}

// NewEnvResolver returns an environment-backed credential resolver.
func NewEnvResolver() EnvResolver { return EnvResolver{} }

// Resolve returns the credential map for a provider, keyed as the providers
// expect (e.g. "user", "password").
func (EnvResolver) Resolve(_ context.Context, p db.Provider) (map[string]string, error) {
	switch p.Type {
	case "vsphere":
		return map[string]string{
			"user":     os.Getenv("OPORD_VSPHERE_USER"),
			"password": os.Getenv("OPORD_VSPHERE_PASSWORD"),
		}, nil
	case "proxmox":
		return map[string]string{
			"user":     os.Getenv("OPORD_PROXMOX_USER"),
			"password": os.Getenv("OPORD_PROXMOX_PASSWORD"),
		}, nil
	default:
		return map[string]string{}, nil
	}
}

// ResolveConfig is a no-op for the env backend: provider config stays in the
// OPORD DB (the env resolver only supplies credentials).
func (EnvResolver) ResolveConfig(_ context.Context, _ db.Provider) (map[string]any, error) {
	return nil, nil
}

// ReadSecret is a no-op for the env backend (no Vault to read from).
func (EnvResolver) ReadSecret(_ context.Context, _ string) (map[string]string, error) {
	return nil, nil
}
