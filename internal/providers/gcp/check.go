package gcp

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CheckConnection implements providers.Connectivity for GCP. It signs a JWT with
// the service-account private key, exchanges it for an OAuth2 access token, then
// GETs the configured project from the Cloud Resource Manager API to confirm the
// SA can see it. Uses only the stdlib (crypto/rsa JWT signing + net/http) - no
// new SDK deps. Read-only: token exchange + a project metadata GET.
func (p *Provider) CheckConnection(ctx context.Context, creds map[string]string, config map[string]any) error {
	// Preferred: a short-lived OAuth2 token minted by the OpenBao GCP secrets
	// engine (ADR-0010). Probe the project with it directly - no JWT to sign.
	if tok := creds["access_token"]; tok != "" {
		project := cfgString(config, "project_id")
		if project == "" {
			project = creds["project_id"]
		}
		if project == "" {
			p.log.Info("gcp: dynamic access token present (no project to probe)")
			return nil
		}
		tctx, tcancel := context.WithTimeout(ctx, 10*time.Second)
		defer tcancel()
		return gcpProbeProject(tctx, tok, project)
	}

	saJSON := gcpCredKeys(creds)
	if saJSON == "" {
		// No SA key: fall back to Application Default Credentials (ADC). This is
		// the keyless path for projects whose org policy blocks SA key creation
		// (iam.disableServiceAccountKeyCreation) - the operator runs
		// `gcloud auth application-default login` and tofu uses ADC directly.
		// Verify ADC is actually present so `provider check` is honest.
		if path := adcPath(); path != "" {
			p.log.Info("gcp: no SA key - using Application Default Credentials", "adc", path)
			return nil
		}
		return fmt.Errorf("gcp: no credentials - set this provider's secret-ref to an OpenBao path with the SA JSON (key `credentials`), OR run `gcloud auth application-default login` for keyless ADC")
	}
	var sa struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
		ProjectID   string `json:"project_id"`
	}
	if err := json.Unmarshal([]byte(saJSON), &sa); err != nil {
		return fmt.Errorf("gcp: service-account key is not valid JSON: %w", err)
	}
	if sa.ClientEmail == "" || sa.PrivateKey == "" {
		return fmt.Errorf("gcp: service-account key is missing client_email / private_key")
	}
	tokenURI := sa.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	token, err := gcpAccessToken(ctx, sa.ClientEmail, sa.PrivateKey, tokenURI)
	if err != nil {
		return err
	}

	project := cfgString(config, "project_id")
	if project == "" {
		project = firstNonEmpty(creds["project_id"], sa.ProjectID)
	}
	if project == "" {
		// Token alone proves the key is valid; without a project we can't probe further.
		return nil
	}
	return gcpProbeProject(ctx, token, project)
}

// gcpAccessToken signs a service-account JWT and exchanges it for an OAuth2
// access token (the standard two-legged SA flow).
func gcpAccessToken(ctx context.Context, clientEmail, privateKeyPEM, tokenURI string) (string, error) {
	key, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return "", err
	}
	now := time.Now()
	header := b64url([]byte(`{"alg":"RS256","typ":"JWT"}`))
	claims := map[string]any{
		"iss":   clientEmail,
		"scope": "https://www.googleapis.com/auth/cloud-platform",
		"aud":   tokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	cb, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("gcp: marshal jwt claims: %w", err)
	}
	signingInput := header + "." + b64url(cb)
	h := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	if err != nil {
		return "", fmt.Errorf("gcp: sign jwt: %w", err)
	}
	assertion := signingInput + "." + b64url(sig)

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", assertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("gcp: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gcp: token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var ge struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &ge)
		if ge.Error != "" {
			return "", fmt.Errorf("gcp: auth failed (%s): %s", ge.Error, ge.ErrorDescription)
		}
		return "", fmt.Errorf("gcp: auth failed (HTTP %d)", resp.StatusCode)
	}
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("gcp: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("gcp: empty access_token in response")
	}
	return tr.AccessToken, nil
}

// gcpProbeProject confirms the SA can read the configured project. 200 = OK; 403
// = token valid but the SA lacks access; 404 = wrong project id.
func gcpProbeProject(ctx context.Context, token, project string) error {
	endpoint := "https://cloudresourcemanager.googleapis.com/v1/projects/" + url.PathEscape(project)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("gcp: build project request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("gcp: project probe: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusForbidden:
		return fmt.Errorf("gcp: service account has no access to project %q, or the Cloud Resource Manager API is not enabled", project)
	case http.StatusNotFound:
		return fmt.Errorf("gcp: project %q not found (check provider config project_id)", project)
	default:
		var ge struct {
			Error struct {
				Message string `json:"message"`
				Status  string `json:"status"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &ge)
		if ge.Error.Message != "" {
			return fmt.Errorf("gcp: project probe failed (%s): %s", ge.Error.Status, ge.Error.Message)
		}
		return fmt.Errorf("gcp: project probe failed (HTTP %d)", resp.StatusCode)
	}
}

// parseRSAPrivateKey parses a PEM-encoded PKCS#8 (or PKCS#1) RSA private key
// from a service-account key file.
func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("gcp: service-account private_key is not valid PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rk, ok := key.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, fmt.Errorf("gcp: service-account key is not an RSA key")
	}
	if rk, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return rk, nil
	}
	return nil, fmt.Errorf("gcp: could not parse the service-account private key")
}

// b64url is the JWT base64url (no padding) encoding.
func b64url(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// adcPath returns the path to Application Default Credentials if present, else
// "". Checks GOOGLE_APPLICATION_CREDENTIALS first, then the gcloud well-known
// file - the same order the google tofu provider uses. Lets the keyless path
// (gcloud auth application-default login) work for projects that block SA keys.
func adcPath() string {
	if p := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); p != "" {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	wellKnown := filepath.Join(home, ".config", "gcloud", "application_default_credentials.json")
	if _, err := os.Stat(wellKnown); err == nil {
		return wellKnown
	}
	return ""
}
