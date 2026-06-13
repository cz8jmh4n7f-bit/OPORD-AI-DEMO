package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

// fakeAnthropic emulates the key split: the ADMIN key only works on
// /v1/organizations/*, the inference key only on /v1/models - exactly the live
// behavior that motivated the two-credential check (ADR-0022).
func fakeAnthropic(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("x-api-key")
		calls = append(calls, r.URL.Path+"|"+key)
		switch {
		case strings.HasPrefix(r.URL.Path, "/v1/organizations/"):
			if key != "sk-ant-admin-good" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		case r.URL.Path == "/v1/models":
			if key != "sk-ant-api03-good" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func check(srv *httptest.Server, creds map[string]string) error {
	p := Provider{client: srv.Client()}
	return p.ValidateCredentials(context.Background(), aiproviders.CredentialRequest{
		Credentials: creds,
		Config:      map[string]any{"base_url": srv.URL},
	})
}

func TestValidateCredentialsAdminOnly(t *testing.T) {
	srv, calls := fakeAnthropic(t)
	// Governance-only setup: just the admin key. Must check green by probing the
	// Admin API - NOT /v1/models (where an admin key 401s).
	if err := check(srv, map[string]string{"admin_api_key": "sk-ant-admin-good"}); err != nil {
		t.Fatalf("admin-only check failed: %v", err)
	}
	for _, c := range *calls {
		if strings.HasPrefix(c, "/v1/models") {
			t.Fatalf("admin-only check must not probe /v1/models, calls: %v", *calls)
		}
	}
}

func TestValidateCredentialsBothKeys(t *testing.T) {
	srv, calls := fakeAnthropic(t)
	if err := check(srv, map[string]string{
		"admin_api_key": "sk-ant-admin-good",
		"api_key":       "sk-ant-api03-good",
	}); err != nil {
		t.Fatalf("both-keys check failed: %v", err)
	}
	var org, models bool
	for _, c := range *calls {
		if strings.HasPrefix(c, "/v1/organizations/") {
			org = true
		}
		if strings.HasPrefix(c, "/v1/models") {
			models = true
		}
	}
	if !org || !models {
		t.Fatalf("both keys must probe both surfaces (org=%v models=%v): %v", org, models, *calls)
	}
}

func TestValidateCredentialsBadAdminNamed(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	err := check(srv, map[string]string{"admin_api_key": "sk-ant-admin-WRONG"})
	if err == nil || !strings.Contains(err.Error(), "admin key") {
		t.Fatalf("bad admin key must fail naming the admin key, got: %v", err)
	}
}

func TestValidateCredentialsMissingBoth(t *testing.T) {
	srv, _ := fakeAnthropic(t)
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_ADMIN_KEY", "")
	err := check(srv, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "admin_api_key") {
		t.Fatalf("missing keys must explain the two-credential layout, got: %v", err)
	}
}

func TestListModelsCuratedFallbackWithoutInferenceKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	p := Provider{}
	models, err := p.ListModels(context.Background(), aiproviders.ModelListRequest{
		Credentials: map[string]string{"admin_api_key": "sk-ant-admin-good"}, // governance-only
	})
	if err != nil {
		t.Fatalf("curated fallback errored: %v", err)
	}
	ids := map[string]bool{}
	for _, m := range models {
		ids[m.Model] = true
	}
	for _, want := range []string{"claude-opus-4-8", "claude-sonnet-4-6", "claude-haiku-4-5"} {
		if !ids[want] {
			t.Fatalf("curated list missing %s: %v", want, ids)
		}
	}
	if ids["claude-opus-latest"] {
		t.Fatal("invented -latest placeholders must be gone")
	}
}

func TestListModelsLivePaginated(t *testing.T) {
	pageCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Header.Get("x-api-key") != "sk-ant-api03-good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		pageCalls++
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("after_id") == "" {
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-opus-4-8","display_name":"Claude Opus 4.8","created_at":"2026-02-01T00:00:00Z"}],"has_more":true,"last_id":"claude-opus-4-8"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-haiku-4-5","display_name":"Claude Haiku 4.5","created_at":"2025-10-01T00:00:00Z"}],"has_more":false,"last_id":"claude-haiku-4-5"}`))
	}))
	defer srv.Close()

	p := Provider{client: srv.Client()}
	models, err := p.ListModels(context.Background(), aiproviders.ModelListRequest{
		Credentials: map[string]string{"api_key": "sk-ant-api03-good"},
		Config:      map[string]any{"base_url": srv.URL},
	})
	if err != nil {
		t.Fatalf("live list errored: %v", err)
	}
	if pageCalls != 2 {
		t.Fatalf("expected 2 paginated calls, got %d", pageCalls)
	}
	if len(models) != 2 || models[0].Model != "claude-opus-4-8" || models[1].Model != "claude-haiku-4-5" {
		t.Fatalf("unexpected models: %+v", models)
	}
	if models[0].Metadata["source"] != "anthropic_models_api" {
		t.Fatalf("live source tag missing: %+v", models[0].Metadata)
	}
}
