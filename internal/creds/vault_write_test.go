package creds

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestVaultResolver_WriteSecret verifies a generated secret (e.g. a managed-DB
// master password) is PUT to the KV v2 store at the given path with the supplied
// data - the path OPORD uses instead of leaving the password only in tofu state.
func TestVaultResolver_WriteSecret(t *testing.T) {
	var gotPath string
	var gotData map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if (r.Method == http.MethodPut || r.Method == http.MethodPost) && strings.HasSuffix(r.URL.Path, "/v1/secret/data/opord/databases/app-db") {
			gotPath = r.URL.Path
			var body struct {
				Data map[string]any `json:"data"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotData = body.Data
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"version": 1}})
			return
		}
		http.Error(w, `{"errors":["not found"]}`, http.StatusNotFound)
	}))
	defer srv.Close()

	r := NewResolver(srv.URL, "test-token", "secret", nil)
	vr, ok := r.(VaultResolver)
	if !ok {
		t.Fatalf("expected a VaultResolver, got %T", r)
	}

	err := vr.WriteSecret(context.Background(), "opord/databases/app-db", map[string]string{
		"password": "s3cr3t-pw",
		"username": "appuser",
	})
	if err != nil {
		t.Fatalf("WriteSecret: %v", err)
	}
	if !strings.HasSuffix(gotPath, "/v1/secret/data/opord/databases/app-db") {
		t.Fatalf("unexpected write path: %q", gotPath)
	}
	if gotData["password"] != "s3cr3t-pw" || gotData["username"] != "appuser" {
		t.Fatalf("write payload mismatch: %+v", gotData)
	}
}
