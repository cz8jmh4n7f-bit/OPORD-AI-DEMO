package creds

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
)

// Resolver is the credential-resolution shape shared by the env and Vault
// backends. It matches orchestrator.CredentialResolver structurally so either
// can be handed to the Service.
type Resolver interface {
	Resolve(ctx context.Context, p db.Provider) (map[string]string, error)
	// ResolveConfig returns NON-credential keys stored alongside the credentials
	// at the provider's SecretRef (e.g. region, subnet_ids, ou_id) so Vault can be
	// the single source of truth for provider config too. Arrays (subnet_ids) are
	// preserved as []any. Env-only backends return nil (config stays in the DB).
	ResolveConfig(ctx context.Context, p db.Provider) (map[string]any, error)
	// ReadSecret reads an arbitrary KV v2 secret as a string map, so creds that
	// are not tied to a provider row (e.g. Microsoft Graph at "opord/azure/graph")
	// can still come from Vault. The env backend returns (nil, nil).
	ReadSecret(ctx context.Context, path string) (map[string]string, error)
}

// credKeys are the credential field names ResolveConfig excludes - everything
// else stored at the SecretRef is treated as provider config.
var credKeys = map[string]bool{
	"access_key": true, "access_key_id": true, "aws_access_key_id": true,
	"secret_key": true, "secret_access_key": true, "aws_secret_access_key": true,
	"session_token": true, "aws_session_token": true,
	"user": true, "username": true, "password": true,
	"token": true, "app_token": true, "user_token": true,
	// GCP: the SA JSON key, dynamic-token pointer, and minted token are credentials
	// (or credential plumbing), never provider config.
	"credentials": true, "service_account_json": true, "google_credentials": true,
	"gcp_token_path": true, "access_token": true,
	// Azure SP secret + the AWS/Azure dynamic-creds pointers (ADR-0010).
	"client_id": true, "client_secret": true,
	"aws_creds_path": true, "azure_creds_path": true,
}

// secretWaitKey gates the Azure dynamic-secret settle wait. It's set only on the
// PROVISION/DESTROY path (the worker, which runs a real tofu apply/destroy and
// needs creds that work immediately), NOT on interactive paths (HTTP create
// preflight, provider check) which would otherwise block ~90-150s.
type secretWaitKey struct{}

// WithSecretWait marks ctx so the resolver waits for a freshly-minted Azure SP
// secret to propagate before returning. Call it on the apply/destroy path.
func WithSecretWait(ctx context.Context) context.Context {
	return context.WithValue(ctx, secretWaitKey{}, true)
}

func wantSecretWait(ctx context.Context) bool {
	v, _ := ctx.Value(secretWaitKey{}).(bool)
	return v
}

// factoryCredsKey marks an account-factory operation. Its cross-account hops
// (L2-L5 assume_role into the new member account) need creds that can chain
// sts:AssumeRole - which a GetFederationToken session CANNOT (an AWS STS
// restriction). When set AND the provider's SecretRef pins an assumed_role
// engine path under "aws_factory_creds_path", Resolve mints from THAT instead
// of the catalog's federation_token path, so the catalog stays least-privilege
// (ADR-0010 / aws-account-vault-setup runbook).
type factoryCredsKey struct{}

// WithFactoryCreds marks ctx so the AWS resolver prefers the factory's
// assumed_role engine path. Call it on the account provision/destroy path.
func WithFactoryCreds(ctx context.Context) context.Context {
	return context.WithValue(ctx, factoryCredsKey{}, true)
}

func wantFactoryCreds(ctx context.Context) bool {
	v, _ := ctx.Value(factoryCredsKey{}).(bool)
	return v
}

// azureCredCache caches a settled Azure dynamic SP secret per SecretRef so the
// ~90-150s propagation wait is paid ONCE per ~lease (not per operation). Once a
// settle completes, every subsequent resolve - provider check, create preflight,
// provision - reuses the already-propagated secret instantly + reliably. The
// engine's role TTL is ~1h, so caching well under that is safe.
type azureCredCache struct {
	mu sync.Mutex
	m  map[string]cachedAzureCred
}

type cachedAzureCred struct {
	clientID, clientSecret string
	expiry                 time.Time
}

func newAzureCredCache() *azureCredCache {
	return &azureCredCache{m: map[string]cachedAzureCred{}}
}

func (c *azureCredCache) get(key string) (string, string, bool) {
	if c == nil {
		return "", "", false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[key]
	if !ok || nowFunc().After(v.expiry) {
		return "", "", false
	}
	return v.clientID, v.clientSecret, true
}

func (c *azureCredCache) put(key, id, secret string, ttl time.Duration) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cachedAzureCred{clientID: id, clientSecret: secret, expiry: nowFunc().Add(ttl)}
}

// nowFunc is time.Now, overridable in tests.
var nowFunc = time.Now

// azureCacheTTL is how long a settled secret is reused before re-minting.
var azureCacheTTL = 45 * time.Minute

// azureAuthorityHost is the Azure AD authority used to probe a freshly-minted SP
// secret for usability (waitAzureSecretReady). azureProbeInterval is the gap
// between probes; azureSettleDuration is how long the secret must stay
// continuously usable before we trust it has propagated to ALL AAD replicas
// (a single success only proves one replica has it). All overridable in tests so
// they stay hermetic + fast.
var (
	azureAuthorityHost  = "https://login.microsoftonline.com"
	azureProbeInterval  = 6 * time.Second
	azureSettleDuration = 90 * time.Second
)

// VaultResolver reads provider credentials from Vault KV v2 at the provider's
// SecretRef path (e.g. "opord/vsphere/dev"). It falls back to the environment
// when SecretRef is empty or the secret can't be read, so a missing Vault entry
// never hard-blocks a dev flow.
type VaultResolver struct {
	client   *vaultapi.Client
	mount    string
	log      *slog.Logger
	fallback EnvResolver
	azCache  *azureCredCache
}

// NewResolver returns a Vault-backed resolver when addr+token are set and a
// client can be built; otherwise the environment-backed resolver. This is the
// single entry point the commands use to pick a credential backend.
func NewResolver(addr, token, kvMount string, log *slog.Logger) Resolver {
	if log == nil {
		log = slog.Default()
	}
	if addr == "" || token == "" {
		return EnvResolver{}
	}
	cfg := vaultapi.DefaultConfig()
	cfg.Address = addr
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		log.Warn("vault client init failed; using env credentials", "err", err)
		return EnvResolver{}
	}
	client.SetToken(token)
	if kvMount == "" {
		kvMount = "secret"
	}
	log.Info("vault credential resolver enabled", "addr", addr, "mount", kvMount)
	return VaultResolver{client: client, mount: kvMount, log: log, azCache: newAzureCredCache()}
}

// Resolve reads the provider's secret from Vault KV v2, mapping the stored keys
// (e.g. "user", "password") straight through to what the providers expect.
func (r VaultResolver) Resolve(ctx context.Context, p db.Provider) (map[string]string, error) {
	if p.SecretRef == "" {
		return r.fallback.Resolve(ctx, p)
	}
	secret, err := r.client.KVv2(r.mount).Get(ctx, p.SecretRef)
	if err != nil {
		r.log.Warn("vault read failed; falling back to env", "path", p.SecretRef, "err", err)
		return r.fallback.Resolve(ctx, p)
	}
	out := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		if s, ok := v.(string); ok {
			out[k] = s
		} else {
			out[k] = fmt.Sprintf("%v", v)
		}
	}
	// Dynamic credentials (ADR-0010): when the SecretRef holds a pointer to an
	// OpenBao secrets-engine endpoint, read short-lived creds from there instead
	// of using long-lived static keys. The engine bootstrap (its own cloud
	// identity) lives in OpenBao; OPORD/workers only ever see ephemeral creds.
	switch p.Type {
	case "gcp":
		// gcp_token_path to gcp/static-account|roleset/<name>/token to OAuth2 token.
		if path := out["gcp_token_path"]; path != "" {
			if d, err := r.readDynamic(ctx, path); err != nil {
				r.log.Warn("gcp dynamic creds read failed; falling back to static / ADC", "path", path, "err", err)
			} else if tok := dynStr(d, "token"); tok != "" {
				out["access_token"] = tok
			}
		}
	case "aws":
		// aws_creds_path to aws/creds/<role> to access_key + secret_key + STS token.
		// The account factory's cross-account hops need an assumed_role path that
		// CAN chain sts:AssumeRole (federation_token cannot) - prefer
		// aws_factory_creds_path when a factory op marks ctx via WithFactoryCreds.
		path := out["aws_creds_path"]
		if wantFactoryCreds(ctx) {
			if fp := out["aws_factory_creds_path"]; fp != "" {
				path = fp
			}
		}
		if path != "" {
			if d, err := r.readDynamic(ctx, path); err != nil {
				r.log.Warn("aws dynamic creds read failed; falling back to static / env", "path", path, "err", err)
			} else {
				if v := dynStr(d, "access_key"); v != "" {
					out["access_key"] = v
				}
				if v := dynStr(d, "secret_key"); v != "" {
					out["secret_key"] = v
				}
				// The AWS engine names the STS token "security_token"; the AWS
				// provider's env mapper reads session_token.
				if v := dynStr(d, "security_token"); v != "" {
					out["session_token"] = v
				}
			}
		}
	case "azure":
		// azure_creds_path to azure/creds/<role> to a fresh service-principal
		// client_id + client_secret (tenant_id stays from the static config).
		if path := out["azure_creds_path"]; path != "" {
			if cid, sec, ok := r.azCache.get(p.SecretRef); ok {
				// Cache hit: a previously-settled secret. Reuse it instantly +
				// reliably (it has already propagated to all AAD replicas) - no
				// mint, no wait. This is what makes the provider check + every
				// post-warm-up operation fast AND green.
				out["client_id"], out["client_secret"] = cid, sec
			} else if d, err := r.readDynamic(ctx, path); err != nil {
				r.log.Warn("azure dynamic creds read failed; falling back to static", "path", path, "err", err)
				out["azure_dynamic_error"] = err.Error()
			} else {
				cid := dynStr(d, "client_id")
				sec := dynStr(d, "client_secret")
				if cid != "" {
					out["client_id"] = cid
				}
				if sec != "" {
					out["client_secret"] = sec
				}
				if cid == "" || sec == "" {
					out["azure_dynamic_error"] = "dynamic credentials response did not include client_id/client_secret"
				}
				// Azure-specific settle window: the engine adds the SP password
				// via Graph, but Azure AD takes ~30s-2min to propagate it before
				// it can mint a token (AADSTS7000215 "Invalid client secret" until
				// then). AWS STS / GCP OAuth tokens are usable immediately; an
				// Azure app password is not. The wait paths (apply/destroy +
				// provider check, via WithSecretWait) settle it ONCE, then CACHE
				// it so every later resolve - including the HTTP preflight - reuses
				// it without waiting.
				if ten := out["tenant_id"]; cid != "" && sec != "" && ten != "" && wantSecretWait(ctx) {
					// Detach the settle from the request ctx so a provider check
					// that times out in the browser still finishes settling +
					// warms the cache - the next call is then instant + green.
					r.waitAzureSecretReady(context.WithoutCancel(ctx), ten, cid, sec)
					r.azCache.put(p.SecretRef, cid, sec, azureCacheTTL)
				}
			}
		}
	}
	return out, nil
}

// waitAzureSecretReady polls Azure AD's token endpoint until a freshly-minted
// service-principal client secret can mint a token, or a short deadline passes.
// This hides the Azure app-password propagation delay (ADR-0010) so a dynamic
// Azure provision/check doesn't fail on the first immediate use.
func (r VaultResolver) waitAzureSecretReady(ctx context.Context, tenant, clientID, clientSecret string) {
	endpoint := azureAuthorityHost + "/" + tenant + "/oauth2/v2.0/token"
	body := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {"https://management.azure.com/.default"},
		"grant_type":    {"client_credentials"},
	}.Encode()
	probe := func() bool {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
		if err != nil {
			return false
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		ok := resp.StatusCode == http.StatusOK
		resp.Body.Close()
		return ok
	}
	// Azure AD is eventually consistent across replicas with a FLAPPING window: a
	// freshly-added SP password authenticates on one replica before another, so
	// "continuously usable" is never satisfied early (a flap resets it). Instead:
	// (1) wait until the secret first authenticates anywhere, then (2) sleep a
	// fixed settle so it propagates to ALL replicas before tofu (load-balanced
	// across the same front ends) uses it. River still retries the rare residual
	// race (AADSTS7000215 is classified transient).
	firstDeadline := time.Now().Add(90 * time.Second)
	for !probe() {
		if time.Now().After(firstDeadline) {
			r.log.Warn("azure dynamic secret never authenticated within first-wait; returning anyway (River retries AADSTS7000215)")
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(azureProbeInterval):
		}
	}
	// Authenticated on at least one replica - give it the global settle window.
	select {
	case <-ctx.Done():
	case <-time.After(azureSettleDuration):
	}
}

// readDynamic does a logical (dynamic) read of an OpenBao secrets-engine
// endpoint (NOT KV v2) and returns its data map.
func (r VaultResolver) readDynamic(ctx context.Context, path string) (map[string]any, error) {
	secret, err := r.client.Logical().ReadWithContext(ctx, path)
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.Data == nil {
		return nil, fmt.Errorf("no data at dynamic path %q", path)
	}
	return secret.Data, nil
}

// dynStr reads a string field from a dynamic-secret data map (tolerant of types).
func dynStr(d map[string]any, key string) string {
	if v, ok := d[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ResolveConfig returns the non-credential keys stored at the SecretRef as
// provider config (region, subnet_ids, ou_id, …), preserving JSON types. Empty
// SecretRef or a read error yields nil (the DB config is then authoritative).
func (r VaultResolver) ResolveConfig(ctx context.Context, p db.Provider) (map[string]any, error) {
	if p.SecretRef == "" {
		return nil, nil
	}
	secret, err := r.client.KVv2(r.mount).Get(ctx, p.SecretRef)
	if err != nil {
		r.log.Warn("vault read failed; provider config from DB only", "path", p.SecretRef, "err", err)
		return nil, nil
	}
	out := make(map[string]any, len(secret.Data))
	for k, v := range secret.Data {
		if credKeys[k] {
			continue
		}
		out[k] = v
	}
	return out, nil
}

// ReadSecret reads an arbitrary KV v2 path (e.g. "opord/azure/graph") as a
// string map. Empty path returns nil; a read error is returned to the caller.
func (r VaultResolver) ReadSecret(ctx context.Context, path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}
	secret, err := r.client.KVv2(r.mount).Get(ctx, path)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		if s, ok := v.(string); ok {
			out[k] = s
		} else {
			out[k] = fmt.Sprintf("%v", v)
		}
	}
	return out, nil
}

// WriteSecret stores data at the given KV v2 path under the resolver's mount. Used
// to persist a generated secret (e.g. a managed-DB master password) in the secrets
// store instead of leaving it only in tofu state. The env resolver has no store and
// does not implement this; callers type-assert for the capability.
func (r VaultResolver) WriteSecret(ctx context.Context, path string, data map[string]string) error {
	m := make(map[string]any, len(data))
	for k, v := range data {
		m[k] = v
	}
	if _, err := r.client.KVv2(r.mount).Put(ctx, path, m); err != nil {
		return fmt.Errorf("write secret %q: %w", path, err)
	}
	return nil
}
