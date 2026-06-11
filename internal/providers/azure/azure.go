// Package azure implements an infrastructure provider for Microsoft Azure.
// Surfaces: standalone VM (modules/azure-vm), managed PostgreSQL Flexible
// Server (modules/azure-postgres) via the DatabaseProvisioner capability,
// managed Kubernetes (modules/azure-aks) via the k8s-shaped Provider methods,
// and connectivity probe via the Connectivity capability.
//
// Auth: the azurerm Terraform/OpenTofu provider reads ARM_TENANT_ID /
// ARM_CLIENT_ID / ARM_CLIENT_SECRET / ARM_SUBSCRIPTION_ID from the environment.
// OPORD populates these from the provider's resolved credentials (typically a
// service-principal stored at OpenBao path opord/azure/<env>) plus the
// subscription_id from the provider config.
package azure

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// Config configures the Azure provider.
type Config struct {
	ModulesDir   string
	TofuBin      string
	StateConnStr string
	Logger       *slog.Logger
}

// Provider wraps the Azure OpenTofu modules.
type Provider struct {
	cfg             Config
	vmModuleDir     string
	dbModuleDir     string
	aksModuleDir    string
	cosmosModuleDir string
	fnModuleDir     string
	// Subscription factory (ADR-0009): the seven layers + companions.
	subscriptionModDir         string
	subscriptionBaselineModDir string
	subscriptionRBACModDir     string
	secureVNetModDir           string
	securityHardeningModDir    string
	policyModDir               string
	keyVaultBaselineModDir     string
	log                        *slog.Logger
}

var (
	_ providers.Provider            = (*Provider)(nil)
	_ providers.VMProvisioner       = (*Provider)(nil)
	_ providers.DatabaseProvisioner = (*Provider)(nil)
	_ providers.StackProvisioner    = (*Provider)(nil)
	_ providers.TableProvisioner    = (*Provider)(nil)
	_ providers.FunctionProvisioner = (*Provider)(nil)
	_ providers.AccountProvisioner  = (*Provider)(nil)
	_ providers.S3Provisioner       = (*Provider)(nil)
	_ providers.SecretProvisioner   = (*Provider)(nil)
	_ providers.QueueProvisioner    = (*Provider)(nil)
	_ providers.ProjectProvisioner  = (*Provider)(nil)
	_ providers.CacheProvisioner    = (*Provider)(nil)
)

// New constructs an Azure provider.
func New(cfg Config) *Provider {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Provider{
		cfg:                        cfg,
		vmModuleDir:                filepath.Join(cfg.ModulesDir, "azure-vm"),
		dbModuleDir:                filepath.Join(cfg.ModulesDir, "azure-postgres"),
		aksModuleDir:               filepath.Join(cfg.ModulesDir, "azure-aks"),
		cosmosModuleDir:            filepath.Join(cfg.ModulesDir, "azure-cosmos"),
		fnModuleDir:                filepath.Join(cfg.ModulesDir, "azure-functions"),
		subscriptionModDir:         filepath.Join(cfg.ModulesDir, "azure-subscription"),
		subscriptionBaselineModDir: filepath.Join(cfg.ModulesDir, "azure-subscription-baseline"),
		subscriptionRBACModDir:     filepath.Join(cfg.ModulesDir, "azure-subscription-rbac"),
		secureVNetModDir:           filepath.Join(cfg.ModulesDir, "azure-secure-vnet"),
		securityHardeningModDir:    filepath.Join(cfg.ModulesDir, "azure-security-hardening"),
		policyModDir:               filepath.Join(cfg.ModulesDir, "azure-policy"),
		keyVaultBaselineModDir:     filepath.Join(cfg.ModulesDir, "azure-keyvault-baseline"),
		log:                        log,
	}
}

// Register adds the Azure provider factory to a registry.
func Register(reg *providers.Registry, cfg Config) {
	reg.Register(models.ProviderAzure, func() providers.Provider { return New(cfg) })
}

// Type identifies this provider in the registry.
func (p *Provider) Type() models.ProviderType { return models.ProviderAzure }

// The k8s-shaped Provider methods (Validate/Preflight/Plan/Provision/Destroy)
// live in aks.go and wrap modules/azure-aks.

// backendConfig returns the pg-backend block OPORD injects per workspace.
// Identical to other providers - the tofu wrapper consumes it.
func (p *Provider) backendConfig() map[string]string {
	return map[string]string{
		"conn_str": p.cfg.StateConnStr,
	}
}

// azureCredKeys extracts the service-principal credentials from a resolved
// credentials map. Tolerant of common aliases.
func azureCredKeys(creds map[string]string) (tenantID, clientID, clientSecret string) {
	tenantID = firstNonEmpty(creds["tenant_id"], creds["arm_tenant_id"], creds["azure_tenant_id"], creds["tenant"])
	clientID = firstNonEmpty(creds["client_id"], creds["arm_client_id"], creds["azure_client_id"], creds["app_id"], creds["appId"], creds["application_id"])
	clientSecret = firstNonEmpty(creds["client_secret"], creds["arm_client_secret"], creds["azure_client_secret"], creds["password"], creds["secret"])
	return
}

// azureTofuEnv maps resolved credentials + provider config onto the env vars
// the azurerm Terraform/OpenTofu provider reads at apply time.
func azureTofuEnv(creds map[string]string, cfg map[string]any, specLocation string) map[string]string {
	env := map[string]string{}
	tid, cid, sec := azureCredKeys(creds)
	if tid != "" {
		env["ARM_TENANT_ID"] = tid
	}
	if cid != "" {
		env["ARM_CLIENT_ID"] = cid
	}
	if sec != "" {
		env["ARM_CLIENT_SECRET"] = sec
	}
	// subscription_id lives in the provider config (not a secret); fall back
	// to a credential key for convenience if some operator put it there.
	sub := cfgString(cfg, "subscription_id")
	if sub == "" {
		sub = firstNonEmpty(creds["subscription_id"], creds["arm_subscription_id"])
	}
	if sub != "" {
		env["ARM_SUBSCRIPTION_ID"] = sub
	}
	// ARM_USE_CLI=false ensures the provider never falls back to Azure CLI
	// auth (which would only succeed on a developer laptop).
	env["ARM_USE_CLI"] = "false"
	if specLocation != "" {
		env["ARM_LOCATION"] = specLocation
	}
	return env
}

// firstNonEmpty returns the first non-empty string from its arguments.
func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// cfgString reads a string-valued config key (tolerant of nil maps).
func cfgString(cfg map[string]any, key string) string {
	if cfg == nil {
		return ""
	}
	if v, ok := cfg[key].(string); ok {
		return v
	}
	return ""
}

// targetCfg returns the provider config with the Azure deployment subscription
// overridden by a deploy target (ADR-0013) when set - so a catalog resource lands
// in a OPORD-managed subscription (e.g. one the subscription factory created) using
// the provider's OWN service principal, not a fake provider. Empty target = the
// provider's default subscription (unchanged).
func targetCfg(cfg map[string]any, target string) map[string]any {
	if target == "" {
		return cfg
	}
	out := make(map[string]any, len(cfg)+1)
	for k, v := range cfg {
		out[k] = v
	}
	out["subscription_id"] = target
	return out
}

// cfgStringDefault is cfgString with a default value when the key is missing.
func cfgStringDefault(cfg map[string]any, key, def string) string {
	if v := cfgString(cfg, key); v != "" {
		return v
	}
	return def
}

func cfgBoolDefault(cfg map[string]any, key string, def bool) bool {
	if cfg == nil {
		return def
	}
	switch v := cfg[key].(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes", "y", "on":
			return true
		case "false", "0", "no", "n", "off":
			return false
		}
	}
	return def
}

func cfgStringListDefault(cfg map[string]any, key string, def []string) []string {
	if cfg == nil {
		return def
	}
	switch v := cfg[key].(type) {
	case []string:
		if len(v) > 0 {
			return v
		}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return def
}

func azureSafetyProfile(cfg map[string]any) string {
	profile := strings.ToLower(strings.TrimSpace(cfgStringDefault(cfg, "safety_profile", "dev")))
	switch profile {
	case "sandbox", "dev", "prod":
		return profile
	default:
		return "dev"
	}
}

func azureIsProd(cfg map[string]any) bool {
	return azureSafetyProfile(cfg) == "prod"
}
