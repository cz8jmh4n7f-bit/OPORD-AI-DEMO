package azure

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGraphClientFlow exercises the full client against a fake Graph server:
// token acquisition, SP lookup, app-role ensure (+ PATCH), user lookup, role
// assignment, and EnsureAppRole idempotency (existing value to no PATCH).
func TestGraphClientFlow(t *testing.T) {
	var patched, assigned bool
	existingRoleValue := "" // empty => the app starts with no app roles

	mux := http.NewServeMux()
	mux.HandleFunc("/tid/oauth2/v2.0/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("token: want POST, got %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
	})
	mux.HandleFunc("/servicePrincipals", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{{"id": "sp1", "appId": "app-guid"}},
		})
	})
	mux.HandleFunc("/servicePrincipals/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/appRoleAssignedTo") {
			assigned = true
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/applications", func(w http.ResponseWriter, r *http.Request) {
		roles := []map[string]any{}
		if existingRoleValue != "" {
			roles = append(roles, map[string]any{"id": "existing-id", "value": existingRoleValue, "isEnabled": true})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"value": []map[string]any{{"id": "app1", "appId": "app-guid", "appRoles": roles}},
		})
	})
	mux.HandleFunc("/applications/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			patched = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "u1", "userPrincipalName": "alice", "mail": "alice@x.com",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(Config{
		TenantID: "tid", ClientID: "cid", ClientSecret: "sec",
		LoginBase: srv.URL, GraphBase: srv.URL,
	}, slog.Default())
	if !c.Configured() {
		t.Fatal("expected Configured() = true")
	}
	ctx := context.Background()

	sp, err := c.ServicePrincipalByAppID(ctx, "app-guid")
	if err != nil || sp.ID != "sp1" {
		t.Fatalf("ServicePrincipalByAppID: err=%v sp=%+v", err, sp)
	}

	claim := "arn:aws:iam::123:role/Admin,arn:aws:iam::123:saml-provider/x"
	id, err := c.EnsureAppRole(ctx, "app-guid", "AWS Admin", claim)
	if err != nil || id == "" {
		t.Fatalf("EnsureAppRole(new): err=%v id=%q", err, id)
	}
	if !patched {
		t.Fatal("expected a PATCH /applications for a new app role")
	}

	u, err := c.UserByEmail(ctx, "alice@x.com")
	if err != nil || u.ID != "u1" {
		t.Fatalf("UserByEmail: err=%v u=%+v", err, u)
	}

	if err := c.AssignAppRole(ctx, sp.ID, u.ID, id); err != nil {
		t.Fatalf("AssignAppRole: %v", err)
	}
	if !assigned {
		t.Fatal("expected an appRoleAssignedTo POST")
	}

	// Idempotency: when an app role with the same value already exists, reuse its
	// id and do NOT PATCH.
	existingRoleValue = claim
	patched = false
	gotID, err := c.EnsureAppRole(ctx, "app-guid", "AWS Admin", claim)
	if err != nil {
		t.Fatalf("EnsureAppRole(existing): %v", err)
	}
	if gotID != "existing-id" {
		t.Fatalf("expected reuse of existing-id, got %q", gotID)
	}
	if patched {
		t.Fatal("did not expect a PATCH when the app role value already exists")
	}
}

// TestAssignAppRoleRetry verifies AssignAppRole retries the Azure
// eventual-consistency error (an app role added to the application has not yet
// propagated to the service principal) and then succeeds.
func TestAssignAppRoleRetry(t *testing.T) {
	var attempts int
	mux := http.NewServeMux()
	mux.HandleFunc("/tid/oauth2/v2.0/token", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok", "expires_in": 3600})
	})
	mux.HandleFunc("/servicePrincipals/", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"message":"Permission being assigned was not found on application"}}`))
			return
		}
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(Config{TenantID: "tid", ClientID: "c", ClientSecret: "s", LoginBase: srv.URL, GraphBase: srv.URL}, slog.Default())
	if err := c.AssignAppRole(context.Background(), "sp1", "u1", "role1"); err != nil {
		t.Fatalf("expected success after retry, got %v", err)
	}
	if attempts < 2 {
		t.Fatalf("expected a retry (≥2 attempts), got %d", attempts)
	}
}

// TestNotConfigured verifies the guard a Service relies on.
func TestNotConfigured(t *testing.T) {
	c := New(Config{}, slog.Default())
	if c.Configured() {
		t.Fatal("expected Configured() = false for empty config")
	}
}
