// Package gcp implements an infrastructure provider for Google Cloud Platform.
// Surface: VM (gcp-vm), managed Kubernetes (GKE), object storage (GCS), secret
// (Secret Manager), queue (Pub/Sub), cache (Memorystore Redis), database (Cloud
// SQL), serverless function (Cloud Functions gen2), table (Firestore), access
// (project IAM role binding) and the generic Stack (any OpenTofu root), plus a
// connectivity probe - at parity with AWS/Azure on the common primitives. The
// project/subscription-equivalent account factory is the remaining item.
//
// Auth: the OpenTofu `google` provider reads GOOGLE_CREDENTIALS (the service
// account JSON key, as content) + GOOGLE_PROJECT / GOOGLE_REGION / GOOGLE_ZONE
// from the environment. OPORD populates these from the provider's resolved
// credentials (the SA JSON stored at OpenBao path opord/gcp/<env>) plus the
// project_id / region / zone from the provider config. No new Go SDK deps - the
// tofu provider plugin is fetched by tofu; the connectivity check signs a JWT
// with the stdlib.
package gcp

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// Config configures the GCP provider.
type Config struct {
	ModulesDir   string
	TofuBin      string
	StateConnStr string
	Logger       *slog.Logger
}

// Provider wraps the GCP OpenTofu modules.
type Provider struct {
	cfg                Config
	vmModuleDir        string
	gcsModuleDir       string
	secretModuleDir    string
	pubsubModuleDir    string
	redisModuleDir     string
	cloudsqlModuleDir  string
	functionModuleDir  string
	gkeModuleDir       string
	firestoreModuleDir string
	accessModuleDir    string
	log                *slog.Logger
}

var (
	_ providers.Provider            = (*Provider)(nil)
	_ providers.VMProvisioner       = (*Provider)(nil)
	_ providers.S3Provisioner       = (*Provider)(nil)
	_ providers.SecretProvisioner   = (*Provider)(nil)
	_ providers.QueueProvisioner    = (*Provider)(nil)
	_ providers.CacheProvisioner    = (*Provider)(nil)
	_ providers.DatabaseProvisioner = (*Provider)(nil)
	_ providers.FunctionProvisioner = (*Provider)(nil)
	_ providers.TableProvisioner    = (*Provider)(nil)
	_ providers.ProjectProvisioner  = (*Provider)(nil)
	_ providers.StackProvisioner    = (*Provider)(nil)
	_ providers.Connectivity        = (*Provider)(nil)
)

// New constructs a GCP provider.
func New(cfg Config) *Provider {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Provider{
		cfg:                cfg,
		vmModuleDir:        filepath.Join(cfg.ModulesDir, "gcp-vm"),
		gcsModuleDir:       filepath.Join(cfg.ModulesDir, "gcp-gcs"),
		secretModuleDir:    filepath.Join(cfg.ModulesDir, "gcp-secret"),
		pubsubModuleDir:    filepath.Join(cfg.ModulesDir, "gcp-pubsub"),
		redisModuleDir:     filepath.Join(cfg.ModulesDir, "gcp-memorystore"),
		cloudsqlModuleDir:  filepath.Join(cfg.ModulesDir, "gcp-cloudsql"),
		functionModuleDir:  filepath.Join(cfg.ModulesDir, "gcp-function"),
		gkeModuleDir:       filepath.Join(cfg.ModulesDir, "gcp-gke"),
		firestoreModuleDir: filepath.Join(cfg.ModulesDir, "gcp-firestore"),
		accessModuleDir:    filepath.Join(cfg.ModulesDir, "gcp-iam-access"),
		log:                log,
	}
}

// Register adds the GCP provider factory to a registry.
func Register(reg *providers.Registry, cfg Config) {
	reg.Register(models.ProviderGCP, func() providers.Provider { return New(cfg) })
}

// Type identifies this provider in the registry.
func (p *Provider) Type() models.ProviderType { return models.ProviderGCP }

// backendConfig returns the pg-backend block OPORD injects per workspace.
func (p *Provider) backendConfig() map[string]string {
	return map[string]string{"conn_str": p.cfg.StateConnStr}
}

// The k8s-shaped Provider methods (Validate/Preflight/Plan/Provision/Destroy) for
// managed GKE live in gke.go.

// --- credential + env mapping ---

// gcpCredKeys extracts the service-account JSON key from a resolved credentials
// map. Tolerant of common key aliases.
func gcpCredKeys(creds map[string]string) (saJSON string) {
	return firstNonEmpty(
		creds["credentials"],
		creds["service_account_json"],
		creds["google_credentials"],
		creds["key"],
		creds["json"],
	)
}

// gcpTofuEnv maps resolved credentials + provider config onto the env vars the
// google Terraform/OpenTofu provider reads at apply time. Auth precedence:
//  1. access_token (OpenBao dynamic, ADR-0010) to GOOGLE_OAUTH_ACCESS_TOKEN (keyless)
//  2. SA JSON key (credentials)               to GOOGLE_CREDENTIALS
//  3. neither                                 to ambient ADC (gcloud auth …)
func gcpTofuEnv(creds map[string]string, cfg map[string]any, specRegion string) map[string]string {
	env := map[string]string{}
	if tok := creds["access_token"]; tok != "" {
		// Short-lived OAuth2 token minted by the OpenBao GCP secrets engine.
		// The google provider reads GOOGLE_OAUTH_ACCESS_TOKEN.
		env["GOOGLE_OAUTH_ACCESS_TOKEN"] = tok
	} else if sa := gcpCredKeys(creds); sa != "" {
		// GOOGLE_CREDENTIALS accepts the JSON key content directly.
		env["GOOGLE_CREDENTIALS"] = sa
	}
	if proj := cfgString(cfg, "project_id"); proj != "" {
		env["GOOGLE_PROJECT"] = proj
	} else if proj := firstNonEmpty(creds["project_id"], creds["project"]); proj != "" {
		env["GOOGLE_PROJECT"] = proj
	}
	region := specRegion
	if region == "" {
		region = cfgString(cfg, "region")
	}
	if region != "" {
		env["GOOGLE_REGION"] = region
	}
	if zone := cfgString(cfg, "zone"); zone != "" {
		env["GOOGLE_ZONE"] = zone
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

// cfgStringDefault is cfgString with a default value when the key is missing.
func cfgStringDefault(cfg map[string]any, key, def string) string {
	if v := cfgString(cfg, key); v != "" {
		return v
	}
	return def
}

// targetCfg returns the provider config with the GCP deployment project overridden
// by a deploy target (ADR-0013) when set - so a catalog resource lands in a
// OPORD-managed project (e.g. one the account factory created) using the
// provider's OWN credentials, rather than registering that project as a provider.
// Empty target = the provider's default project (unchanged).
func targetCfg(cfg map[string]any, target string) map[string]any {
	if target == "" {
		return cfg
	}
	out := make(map[string]any, len(cfg)+1)
	for k, v := range cfg {
		out[k] = v
	}
	out["project_id"] = target
	return out
}

func gcpSafetyProfile(cfg map[string]any) string {
	profile := strings.ToLower(strings.TrimSpace(cfgStringDefault(cfg, "safety_profile", "dev")))
	switch profile {
	case "sandbox", "dev", "prod":
		return profile
	default:
		return "dev"
	}
}

func gcpIsProd(cfg map[string]any) bool {
	return gcpSafetyProfile(cfg) == "prod"
}

// cfgStringListDefault reads a []string config key (tolerant of []any), or def.
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
