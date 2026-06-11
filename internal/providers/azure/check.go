package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

var _ providers.Connectivity = (*Provider)(nil)

// CheckConnection implements providers.Connectivity for Azure: it acquires an
// ARM access token via the client_credentials OAuth2 flow against Azure AD, then
// (if subscription_id is configured) does a GET on the subscription to confirm
// the SP can reach it. Uses raw net/http - no new SDK deps. Read-only / mutates
// nothing: the token-endpoint is "auth this app" and subscription GET is a
// metadata read.
func (p *Provider) CheckConnection(ctx context.Context, creds map[string]string, config map[string]any) error {
	tenantID, clientID, clientSecret := azureCredKeys(creds)
	if tenantID == "" || clientID == "" || clientSecret == "" {
		if creds["azure_creds_path"] != "" {
			if dynErr := strings.TrimSpace(creds["azure_dynamic_error"]); dynErr != "" {
				return fmt.Errorf("azure: dynamic credentials path %q is configured but did not return usable client_id/client_secret: %s", creds["azure_creds_path"], dynErr)
			}
			return fmt.Errorf("azure: dynamic credentials path %q is configured but did not return usable client_id/client_secret; either fix the OpenBao Azure secrets engine role or store static client_id/client_secret in this provider secret", creds["azure_creds_path"])
		}
		return fmt.Errorf("azure: missing service-principal credentials - set this provider's secret-ref to an OpenBao path with tenant_id / client_id / client_secret")
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	token, err := acquireARMToken(ctx, tenantID, clientID, clientSecret)
	if err != nil {
		return err
	}

	// Token alone proves the SP credentials are valid. If a subscription is
	// configured, probe it too so the operator catches "SP exists but has no
	// access to this subscription" at registration time, not first apply.
	sub := cfgString(config, "subscription_id")
	if sub == "" {
		sub = firstNonEmpty(creds["subscription_id"], creds["arm_subscription_id"])
	}
	if sub != "" {
		if err := probeSubscription(ctx, token, sub); err != nil {
			return err
		}
	}
	return nil
}

// acquireARMToken does an OAuth2 client_credentials grant against Azure AD for
// the ARM (management.azure.com) audience and returns the bearer token.
func acquireARMToken(ctx context.Context, tenantID, clientID, clientSecret string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", "https://management.azure.com/.default")

	endpoint := "https://login.microsoftonline.com/" + url.PathEscape(tenantID) + "/oauth2/v2.0/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("azure: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("azure: token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		// Azure AD's error body has {"error":"invalid_client","error_description":"..."}
		var aer struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &aer)
		if aer.Error != "" {
			// Trim long Azure AD error_description (full URL/correlation ids) to the headline.
			desc := aer.ErrorDescription
			if i := strings.IndexAny(desc, "\r\n"); i > 0 {
				desc = desc[:i]
			}
			return "", fmt.Errorf("azure: auth failed (%s): %s", aer.Error, desc)
		}
		return "", fmt.Errorf("azure: auth failed (HTTP %d)", resp.StatusCode)
	}

	var tr struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("azure: parse token response: %w", err)
	}
	if tr.AccessToken == "" {
		return "", fmt.Errorf("azure: empty access_token in response")
	}
	return tr.AccessToken, nil
}

// probeSubscription confirms the SP can reach the configured subscription. A
// 200 means the token is valid AND the SP has at least Reader on the sub; 403
// means token is fine but the SP lacks access; 404 means wrong subscription_id.
func probeSubscription(ctx context.Context, token, subscriptionID string) error {
	endpoint := "https://management.azure.com/subscriptions/" + url.PathEscape(subscriptionID) + "?api-version=2022-12-01"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("azure: build subscription request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("azure: subscription probe: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusForbidden:
		return fmt.Errorf("azure: service principal has no access to subscription %s (assign Contributor or Reader)", subscriptionID)
	case http.StatusNotFound:
		return fmt.Errorf("azure: subscription %s not found (check provider config)", subscriptionID)
	default:
		// Azure ARM error body: {"error":{"code":"...","message":"..."}}
		var aer struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &aer)
		if aer.Error.Code != "" {
			return fmt.Errorf("azure: subscription probe failed (%s): %s", aer.Error.Code, aer.Error.Message)
		}
		return fmt.Errorf("azure: subscription probe failed (HTTP %d)", resp.StatusCode)
	}
}
