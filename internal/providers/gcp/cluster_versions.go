package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

var _ providers.ClusterVersionLister = (*Provider)(nil)

// gcpToken returns an OAuth2 access token for raw HTTP calls to Google APIs: the
// dynamic OpenBao token if present (ADR-0010), else one signed from the SA key
// (reusing the CheckConnection two-legged flow). Returns an error for the ADC
// keyless path - there's no in-process token to hand to a bare HTTP request, so
// callers degrade to free-text.
func gcpToken(ctx context.Context, creds map[string]string, _ map[string]any) (string, error) {
	if tok := creds["access_token"]; tok != "" {
		return tok, nil
	}
	saJSON := gcpCredKeys(creds)
	if saJSON == "" {
		return "", fmt.Errorf("gcp: no in-process OAuth token (ADC/keyless) - live version list unavailable")
	}
	var sa struct {
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
		TokenURI    string `json:"token_uri"`
	}
	if err := json.Unmarshal([]byte(saJSON), &sa); err != nil {
		return "", fmt.Errorf("gcp: service-account key is not valid JSON: %w", err)
	}
	tokenURI := sa.TokenURI
	if tokenURI == "" {
		tokenURI = "https://oauth2.googleapis.com/token"
	}
	return gcpAccessToken(ctx, sa.ClientEmail, sa.PrivateKey, tokenURI)
}

// ListClusterVersions queries the live GKE serverConfig for the project+location
// and returns the distinct major.minor versions GKE offers RIGHT NOW (newest
// first; GKE accepts a major.minor as min_master_version and resolves the latest
// patch). This replaces a stale hardcoded list - GKE moves fast (e.g. it already
// dropped 1.31 in favour of 1.35 in europe-west1).
func (p *Provider) ListClusterVersions(ctx context.Context, req providers.ClusterVersionRequest) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	token, err := gcpToken(ctx, req.Credentials, req.Config)
	if err != nil {
		return nil, err
	}
	project := cfgString(req.Config, "project_id")
	if project == "" {
		return nil, fmt.Errorf("gcp: no project_id in provider config")
	}
	location := strings.TrimSpace(req.Region)
	if location == "" {
		location = cfgStringDefault(req.Config, "region", "europe-west1")
	}
	url := fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/%s/serverConfig", project, location)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("gcp: GKE serverConfig request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gcp: GKE serverConfig returned HTTP %d", resp.StatusCode)
	}
	var out struct {
		ValidMasterVersions []string `json:"validMasterVersions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	// GKE returns versions newest-first; dedup to major.minor preserving that order.
	seen := map[string]bool{}
	var versions []string
	for _, v := range out.ValidMasterVersions {
		mm := majorMinor(v)
		if mm == "" || seen[mm] {
			continue
		}
		seen[mm] = true
		versions = append(versions, mm)
	}
	return versions, nil
}

// majorMinor reduces a GKE version ("1.35.5-gke.1057000") to "1.35".
func majorMinor(v string) string {
	parts := strings.SplitN(strings.TrimSpace(v), ".", 3)
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "." + parts[1]
}
