package proxmox

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

var _ providers.Connectivity = (*Provider)(nil)

// CheckConnection implements providers.Connectivity by requesting an auth ticket
// from the Proxmox VE API (POST /access/ticket). A 200 proves both reachability
// and that the username/password are valid - without creating anything. The
// endpoint is read from config["endpoint"] (what the provider provisions with),
// falling back to config["server"] (what `opord provider add` records).
func (p *Provider) CheckConnection(ctx context.Context, creds map[string]string, config map[string]any) error {
	base := cfgString(config, "endpoint")
	if base == "" {
		base = cfgString(config, "server")
	}
	if base == "" {
		return fmt.Errorf("proxmox: provider config has neither 'endpoint' nor 'server'")
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}
	base = strings.TrimRight(base, "/")
	if !strings.Contains(base, "/api2/json") {
		base += "/api2/json"
	}

	user, pass := creds["user"], creds["password"]
	if user == "" || pass == "" {
		return fmt.Errorf("proxmox: missing user/password credentials")
	}

	form := url.Values{"username": {user}, "password": {pass}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/access/ticket", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("proxmox: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Proxmox lab nodes typically use self-signed certs; default lenient. Honor
	// "insecure" (what provisioning reads) first, then "allow_unverified_ssl"
	// (vSphere's key) as a fallback.
	insecure := true
	if v, ok := config["insecure"].(bool); ok {
		insecure = v
	} else if v, ok := config["allow_unverified_ssl"].(bool); ok {
		insecure = v
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure}, //nolint:gosec // operator-controlled lab flag
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("proxmox: connecting to %s: %w", base, err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return fmt.Errorf("proxmox: authentication failed (check user/password)")
	default:
		return fmt.Errorf("proxmox: unexpected response (HTTP %d)", resp.StatusCode)
	}
}
