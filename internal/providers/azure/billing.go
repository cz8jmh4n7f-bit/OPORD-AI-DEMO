package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

var _ providers.BillingScopeLister = (*Provider)(nil)

// billingAPIVersion is the GA Microsoft.Billing version that exposes
// billingAccounts / billingProfiles / invoiceSections for MCA.
const billingAPIVersion = "2020-05-01"

// ListBillingScopes implements providers.BillingScopeLister for Azure: it
// enumerates the MCA hierarchy (billing account to billing profile to invoice
// section) the service principal can see and returns each invoice section as a
// pickable billing scope (its resource id is exactly what `azure_billing_scope_id`
// needs for create mode). Best-effort + read-only: any level the SP can't list is
// skipped, and the caller (account form) falls back to manual entry on error or
// an empty result.
func (p *Provider) ListBillingScopes(ctx context.Context, req providers.BillingScopeRequest) ([]providers.BillingScope, error) {
	tenantID, clientID, clientSecret := azureCredKeys(req.Credentials)
	if tenantID == "" || clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("azure: missing service-principal credentials - set this provider's secret-ref to an OpenBao path with tenant_id / client_id / client_secret")
	}

	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	token, err := acquireARMToken(ctx, tenantID, clientID, clientSecret)
	if err != nil {
		return nil, err
	}

	var accounts struct {
		Value []struct {
			Name       string `json:"name"`
			Properties struct {
				DisplayName string `json:"displayName"`
			} `json:"properties"`
		} `json:"value"`
	}
	if err := armGetJSON(ctx, token, "https://management.azure.com/providers/Microsoft.Billing/billingAccounts?api-version="+billingAPIVersion, &accounts); err != nil {
		return nil, err
	}

	out := []providers.BillingScope{}
	for _, ba := range accounts.Value {
		baLabel := firstNonEmpty(ba.Properties.DisplayName, ba.Name)

		var profiles struct {
			Value []struct {
				Name       string `json:"name"`
				Properties struct {
					DisplayName string `json:"displayName"`
				} `json:"properties"`
			} `json:"value"`
		}
		// A profile/section level the SP can't list is skipped (continue, don't fail).
		if err := armGetJSON(ctx, token, "https://management.azure.com/providers/Microsoft.Billing/billingAccounts/"+ba.Name+"/billingProfiles?api-version="+billingAPIVersion, &profiles); err != nil {
			continue
		}
		for _, bp := range profiles.Value {
			bpLabel := firstNonEmpty(bp.Properties.DisplayName, bp.Name)

			var sections struct {
				Value []struct {
					ID         string `json:"id"`
					Name       string `json:"name"`
					Properties struct {
						DisplayName string `json:"displayName"`
					} `json:"properties"`
				} `json:"value"`
			}
			if err := armGetJSON(ctx, token, "https://management.azure.com/providers/Microsoft.Billing/billingAccounts/"+ba.Name+"/billingProfiles/"+bp.Name+"/invoiceSections?api-version="+billingAPIVersion, &sections); err != nil {
				continue
			}
			for _, is := range sections.Value {
				if is.ID == "" {
					continue
				}
				isLabel := firstNonEmpty(is.Properties.DisplayName, is.Name)
				out = append(out, providers.BillingScope{
					ID:   is.ID,
					Name: fmt.Sprintf("%s / %s / %s", baLabel, bpLabel, isLabel),
					Type: "invoiceSection",
				})
			}
		}
	}
	return out, nil
}

// armGetJSON does an authenticated GET against ARM and decodes the body into v.
// It surfaces Azure's structured error (code + message) on non-200.
func armGetJSON(ctx context.Context, token, endpoint string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("azure: build billing request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("azure: billing request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		var aer struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		_ = json.Unmarshal(body, &aer)
		if aer.Error.Code != "" {
			return fmt.Errorf("azure: billing list failed (%s): %s", aer.Error.Code, aer.Error.Message)
		}
		return fmt.Errorf("azure: billing list failed (HTTP %d)", resp.StatusCode)
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("azure: parse billing response: %w", err)
	}
	return nil
}
