// Package config loads OPORD runtime configuration from the environment
// (see .env.example). It applies sensible dev defaults so the binaries run
// out of the box.
package config

import (
	"os"
	"strings"
)

// Config holds all runtime settings for the API, worker, and CLI.
type Config struct {
	HTTPAddr string
	LogLevel string
	Env      string

	DatabaseURL string

	VaultAddr    string
	VaultToken   string
	VaultKVMount string
	VaultKVBase  string

	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURL  string

	ModulesDir            string
	TofuBin               string
	AnsibleBin            string
	AnsibleDir            string
	SSHPrivateKey         string
	ArtifactsDir          string
	ReconcileInterval     string
	ProviderCheckInterval string

	SlackWebhookURL string
	SIEMURL         string

	GLPIURL       string
	GLPIAppToken  string
	GLPIUserToken string
	GLPIItemType  string

	// Microsoft Graph (Entra / Azure AD) - OPORD's own app-registration creds
	// (client-credentials) for automating the SAML-federation user/role side.
	AzureTenantID     string
	AzureClientID     string
	AzureClientSecret string

	AuthEnabled bool
	// SeedDemoUsers, when true, idempotently creates the demo admin + viewer
	// users (with fixed, documented API keys) on API start. Demo only.
	SeedDemoUsers bool
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr: getenv("OPORD_HTTP_ADDR", ":8080"),
		LogLevel: getenv("OPORD_LOG_LEVEL", "info"),
		Env:      getenv("OPORD_ENV", "dev"),

		DatabaseURL: os.Getenv("DATABASE_URL"),

		VaultAddr:    getenv("VAULT_ADDR", "http://localhost:8200"),
		VaultToken:   os.Getenv("VAULT_TOKEN"),
		VaultKVMount: getenv("VAULT_KV_MOUNT", "secret"),
		VaultKVBase:  getenv("VAULT_KV_BASE", "opord"),

		OIDCIssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		OIDCClientID:     os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
		OIDCRedirectURL:  os.Getenv("OIDC_REDIRECT_URL"),

		ModulesDir:            getenv("OPORD_MODULES_DIR", "./modules"),
		TofuBin:               getenv("TOFU_BIN", "tofu"),
		AnsibleBin:            getenv("ANSIBLE_BIN", "ansible-playbook"),
		AnsibleDir:            getenv("ANSIBLE_DIR", "./ansible"),
		SSHPrivateKey:         os.Getenv("OPORD_SSH_PRIVATE_KEY"),
		ArtifactsDir:          getenv("OPORD_ARTIFACTS_DIR", "./artifacts"),
		ReconcileInterval:     getenv("OPORD_RECONCILE_INTERVAL", "15m"),
		ProviderCheckInterval: getenv("OPORD_PROVIDER_CHECK_INTERVAL", "0"),

		SlackWebhookURL: os.Getenv("OPORD_SLACK_WEBHOOK_URL"),
		SIEMURL:         os.Getenv("OPORD_SIEM_URL"),

		GLPIURL:       os.Getenv("OPORD_GLPI_URL"),
		GLPIAppToken:  os.Getenv("OPORD_GLPI_APP_TOKEN"),
		GLPIUserToken: os.Getenv("OPORD_GLPI_USER_TOKEN"),
		GLPIItemType:  getenv("OPORD_GLPI_ITEM_TYPE", "Computer"),

		AzureTenantID:     os.Getenv("AZURE_TENANT_ID"),
		AzureClientID:     os.Getenv("AZURE_CLIENT_ID"),
		AzureClientSecret: os.Getenv("AZURE_CLIENT_SECRET"),

		AuthEnabled:   getenv("OPORD_AUTH_ENABLED", "false") == "true",
		SeedDemoUsers: getenv("OPORD_SEED_DEMO_USERS", "false") == "true",
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
