package gcp

import "testing"

func TestGCPTofuEnv_authPrecedence(t *testing.T) {
	cfg := map[string]any{"project_id": "proj", "region": "us-central1", "zone": "us-central1-a"}

	t.Run("access_token wins over SA key (keyless, ADR-0010)", func(t *testing.T) {
		env := gcpTofuEnv(map[string]string{"access_token": "ya29.tok", "credentials": `{"x":1}`}, cfg, "")
		if env["GOOGLE_OAUTH_ACCESS_TOKEN"] != "ya29.tok" {
			t.Fatalf("expected GOOGLE_OAUTH_ACCESS_TOKEN=ya29.tok, got %q", env["GOOGLE_OAUTH_ACCESS_TOKEN"])
		}
		if _, ok := env["GOOGLE_CREDENTIALS"]; ok {
			t.Fatalf("GOOGLE_CREDENTIALS must NOT be set when an access token is present")
		}
	})

	t.Run("SA key used when no token", func(t *testing.T) {
		env := gcpTofuEnv(map[string]string{"credentials": `{"type":"service_account"}`}, cfg, "")
		if env["GOOGLE_CREDENTIALS"] == "" {
			t.Fatal("expected GOOGLE_CREDENTIALS to be set from the SA key")
		}
		if _, ok := env["GOOGLE_OAUTH_ACCESS_TOKEN"]; ok {
			t.Fatal("no access token should be set")
		}
	})

	t.Run("neither -> ADC fallback (no cred env)", func(t *testing.T) {
		env := gcpTofuEnv(map[string]string{}, cfg, "")
		if _, ok := env["GOOGLE_CREDENTIALS"]; ok {
			t.Fatal("GOOGLE_CREDENTIALS must be unset so the google provider uses ADC")
		}
		if _, ok := env["GOOGLE_OAUTH_ACCESS_TOKEN"]; ok {
			t.Fatal("GOOGLE_OAUTH_ACCESS_TOKEN must be unset")
		}
	})

	t.Run("project + region/zone always mapped", func(t *testing.T) {
		env := gcpTofuEnv(map[string]string{"access_token": "t"}, cfg, "")
		if env["GOOGLE_PROJECT"] != "proj" || env["GOOGLE_REGION"] != "us-central1" || env["GOOGLE_ZONE"] != "us-central1-a" {
			t.Fatalf("project/region/zone mapping wrong: %+v", env)
		}
	})

	t.Run("spec region overrides config region", func(t *testing.T) {
		env := gcpTofuEnv(map[string]string{"access_token": "t"}, cfg, "europe-west1")
		if env["GOOGLE_REGION"] != "europe-west1" {
			t.Fatalf("spec region should win, got %q", env["GOOGLE_REGION"])
		}
	})
}
