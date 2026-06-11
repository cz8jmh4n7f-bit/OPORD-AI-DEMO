package creds

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
)

// TestVaultResolver_GCPDynamicToken verifies that a GCP provider whose SecretRef
// KV holds a `gcp_token_path` pointer gets a short-lived OAuth2 access token
// minted from the OpenBao GCP secrets-engine token endpoint (ADR-0010), surfaced
// as creds["access_token"].
func TestVaultResolver_GCPDynamicToken(t *testing.T) {
	const mintedToken = "ya29.minted-short-lived-token"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		// KV v2 read of the provider SecretRef: a pointer, no key material.
		case strings.HasSuffix(r.URL.Path, "/v1/secret/data/opord/gcp/dev"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"data":     map[string]any{"gcp_token_path": "gcp/static-account/opord/token"},
					"metadata": map[string]any{"version": 1},
				},
			})
		// GCP secrets-engine token endpoint (logical/dynamic read).
		case strings.HasSuffix(r.URL.Path, "/v1/gcp/static-account/opord/token"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"token":              mintedToken,
					"token_ttl":          3600,
					"expires_at_seconds": 1893456000,
				},
			})
		default:
			http.Error(w, `{"errors":["not found"]}`, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	r := NewResolver(srv.URL, "test-token", "secret", nil)
	vr, ok := r.(VaultResolver)
	if !ok {
		t.Fatalf("expected a VaultResolver, got %T", r)
	}

	out, err := vr.Resolve(context.Background(), db.Provider{
		Type:      "gcp",
		SecretRef: "opord/gcp/dev",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if out["access_token"] != mintedToken {
		t.Fatalf("expected access_token=%q, got %q (full: %+v)", mintedToken, out["access_token"], out)
	}
	if out["gcp_token_path"] != "gcp/static-account/opord/token" {
		t.Fatalf("the pointer should still be present, got %q", out["gcp_token_path"])
	}
}

// TestVaultResolver_GCPNoTokenPath verifies a GCP provider WITHOUT a token
// pointer is left untouched (no access_token minted) - back-compat with the SA
// key / ADC paths.
func TestVaultResolver_GCPNoTokenPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/v1/secret/data/opord/gcp/dev") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"data":     map[string]any{"project_id": "proj"},
					"metadata": map[string]any{"version": 1},
				},
			})
			return
		}
		http.Error(w, `{"errors":["not found"]}`, http.StatusNotFound)
	}))
	defer srv.Close()

	vr := NewResolver(srv.URL, "test-token", "secret", nil).(VaultResolver)
	out, err := vr.Resolve(context.Background(), db.Provider{Type: "gcp", SecretRef: "opord/gcp/dev"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if _, ok := out["access_token"]; ok {
		t.Fatal("no access_token should be minted without a gcp_token_path")
	}
}

// TestVaultResolver_AWSDynamicCreds: an aws_creds_path pointer to short-lived AWS
// creds from aws/creds/<role> (access_key + secret_key + STS security_token to 
// session_token).
func TestVaultResolver_AWSDynamicCreds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/secret/data/opord/aws/eu"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"data":     map[string]any{"aws_creds_path": "aws/creds/opord"},
				"metadata": map[string]any{"version": 1},
			}})
		case strings.HasSuffix(r.URL.Path, "/v1/aws/creds/opord"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"access_key": "ASIA_EPHEMERAL", "secret_key": "ephemeral-secret", "security_token": "sts-token",
			}})
		default:
			http.Error(w, `{"errors":["nf"]}`, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	vr := NewResolver(srv.URL, "t", "secret", nil).(VaultResolver)
	out, err := vr.Resolve(context.Background(), db.Provider{Type: "aws", SecretRef: "opord/aws/eu"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if out["access_key"] != "ASIA_EPHEMERAL" || out["secret_key"] != "ephemeral-secret" {
		t.Fatalf("dynamic AWS key/secret wrong: %+v", out)
	}
	if out["session_token"] != "sts-token" {
		t.Fatalf("security_token should map to session_token, got %q", out["session_token"])
	}
}

// TestVaultResolver_AzureDynamicCreds: an azure_creds_path pointer to a fresh SP
// client_id + client_secret from azure/creds/<role>.
func TestVaultResolver_AzureDynamicCreds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/secret/data/opord/azure/dev"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"data":     map[string]any{"tenant_id": "tid", "azure_creds_path": "azure/creds/opord"},
				"metadata": map[string]any{"version": 1},
			}})
		case strings.HasSuffix(r.URL.Path, "/v1/azure/creds/opord"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"client_id": "ephemeral-app-id", "client_secret": "ephemeral-app-secret",
			}})
		// Azure AD token endpoint (waitAzureSecretReady probe) - report the
		// minted secret as immediately usable so the test stays fast + hermetic.
		case strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token"):
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
		default:
			http.Error(w, `{"errors":["nf"]}`, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// Point the AAD authority probe at the mock instead of real
	// login.microsoftonline.com, and shrink the probe interval so the
	// consecutive-success wait is instant.
	prevHost, prevIv, prevSettle := azureAuthorityHost, azureProbeInterval, azureSettleDuration
	azureAuthorityHost, azureProbeInterval, azureSettleDuration = srv.URL, time.Millisecond, 0
	defer func() {
		azureAuthorityHost, azureProbeInterval, azureSettleDuration = prevHost, prevIv, prevSettle
	}()

	vr := NewResolver(srv.URL, "t", "secret", nil).(VaultResolver)
	out, err := vr.Resolve(context.Background(), db.Provider{Type: "azure", SecretRef: "opord/azure/dev"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if out["client_id"] != "ephemeral-app-id" || out["client_secret"] != "ephemeral-app-secret" {
		t.Fatalf("dynamic Azure SP creds wrong: %+v", out)
	}
	if out["tenant_id"] != "tid" {
		t.Fatalf("static tenant_id should be preserved, got %q", out["tenant_id"])
	}
}

// TestVaultResolver_AzureCredCache: on the wait path (WithSecretWait), a settled
// Azure secret is cached so a second resolve reuses it WITHOUT re-minting from
// the engine (what makes the provider check + later operations fast + reliable).
func TestVaultResolver_AzureCredCache(t *testing.T) {
	var mints int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/v1/secret/data/opord/azure/dev"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"data":     map[string]any{"tenant_id": "tid", "azure_creds_path": "azure/creds/opord"},
				"metadata": map[string]any{"version": 1},
			}})
		case strings.HasSuffix(r.URL.Path, "/v1/azure/creds/opord"):
			mints++
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
				"client_id": "app-id", "client_secret": "app-secret",
			}})
		case strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token"):
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
		default:
			http.Error(w, `{"errors":["nf"]}`, http.StatusNotFound)
		}
	}))
	defer srv.Close()

	prevHost, prevIv, prevSettle := azureAuthorityHost, azureProbeInterval, azureSettleDuration
	azureAuthorityHost, azureProbeInterval, azureSettleDuration = srv.URL, time.Millisecond, 0
	defer func() {
		azureAuthorityHost, azureProbeInterval, azureSettleDuration = prevHost, prevIv, prevSettle
	}()

	vr := NewResolver(srv.URL, "t", "secret", nil).(VaultResolver)
	prov := db.Provider{Type: "azure", SecretRef: "opord/azure/dev"}

	// First resolve on the wait path to mints + settles + caches.
	out1, err := vr.Resolve(WithSecretWait(context.Background()), prov)
	if err != nil {
		t.Fatalf("Resolve#1: %v", err)
	}
	if out1["client_secret"] != "app-secret" {
		t.Fatalf("Resolve#1 secret wrong: %+v", out1)
	}
	// Second resolve to cache hit, NO new mint from the engine.
	out2, err := vr.Resolve(WithSecretWait(context.Background()), prov)
	if err != nil {
		t.Fatalf("Resolve#2: %v", err)
	}
	if out2["client_id"] != "app-id" || out2["client_secret"] != "app-secret" {
		t.Fatalf("Resolve#2 (cache) wrong: %+v", out2)
	}
	if mints != 1 {
		t.Fatalf("expected exactly 1 engine mint (then cache), got %d", mints)
	}
}
