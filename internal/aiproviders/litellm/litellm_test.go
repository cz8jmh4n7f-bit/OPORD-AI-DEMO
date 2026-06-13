package litellm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

func TestProvisionMintsScopedKeyAndRevokeDeletes(t *testing.T) {
	var gen map[string]any
	var deletedAliases []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-master" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/key/generate":
			_ = json.NewDecoder(r.Body).Decode(&gen)
			_ = json.NewEncoder(w).Encode(map[string]any{"key": "sk-virtual-xyz"})
		case "/key/delete":
			var body struct {
				KeyAliases []string `json:"key_aliases"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			deletedAliases = body.KeyAliases
			_ = json.NewEncoder(w).Encode(map[string]any{"deleted_keys": body.KeyAliases})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := Provider{client: srv.Client()}
	creds := map[string]string{"master_key": "sk-master"}
	cfg := map[string]any{"base_url": srv.URL}
	exp := time.Now().Add(48 * time.Hour)
	spec, _ := json.Marshal(map[string]any{"metadata": map[string]any{"models": []string{"mock-gpt"}, "max_budget": 10}})

	res, err := p.ProvisionAccess(context.Background(), aiproviders.ProvisionRequest{
		RequestID: uuid.New(), Owner: "data@team.io", Workspace: "default",
		Spec: spec, ExpiresAt: &exp, Credentials: creds, Config: cfg,
	})
	if err != nil {
		t.Fatalf("ProvisionAccess: %v", err)
	}
	// The minted key must be returned for one-time storage, scoped to the request.
	if res.Observed["virtual_key"] != "sk-virtual-xyz" {
		t.Fatalf("minted key not surfaced: %v", res.Observed["virtual_key"])
	}
	if got, _ := gen["models"].([]any); len(got) != 1 || got[0] != "mock-gpt" {
		t.Fatalf("models not scoped on the key: %v", gen["models"])
	}
	if gen["max_budget"].(float64) != 10 {
		t.Fatalf("budget not scoped: %v", gen["max_budget"])
	}
	if !strings.HasPrefix(res.ProviderAccessID, "opord-data-team") {
		t.Fatalf("access id should be the alias: %s", res.ProviderAccessID)
	}

	// Revoke deletes by alias (no raw key needed).
	if err := p.RevokeAccess(context.Background(), aiproviders.RevokeRequest{
		ProviderAccessID: res.ProviderAccessID, Credentials: creds, Config: cfg,
	}); err != nil {
		t.Fatalf("RevokeAccess: %v", err)
	}
	if len(deletedAliases) != 1 || deletedAliases[0] != res.ProviderAccessID {
		t.Fatalf("revoke did not delete the alias: %v", deletedAliases)
	}
}

func TestGovernanceOnlyWithoutMasterKey(t *testing.T) {
	p := Provider{}
	res, err := p.ProvisionAccess(context.Background(), aiproviders.ProvisionRequest{
		RequestID: uuid.New(), Owner: "x@y.io",
	})
	if err != nil {
		t.Fatalf("ProvisionAccess: %v", err)
	}
	if res.Observed["external_provisioning"] != "manual" {
		t.Fatalf("no master key must fall back to governance-only, got %v", res.Observed["external_provisioning"])
	}
}
