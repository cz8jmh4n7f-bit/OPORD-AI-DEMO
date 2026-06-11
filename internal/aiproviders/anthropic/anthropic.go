// Package anthropic implements OPORD's Anthropic / Claude governance provider.
package anthropic

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

// Provider grants governance records for Anthropic Claude access. MVP
// provisioning is manual: OPORD tracks approval, ownership, expiry, revoke, and
// audit; the real seat/API grant remains in Anthropic/admin tooling.
type Provider struct {
	client *http.Client
}

// Register wires AnthropicProvider into the AI provider registry.
func Register(r *aiproviders.Registry) {
	r.Register(aiproviders.ProviderAnthropic, func() aiproviders.Provider {
		return Provider{client: &http.Client{Timeout: 10 * time.Second}}
	})
}

func (Provider) Type() aiproviders.ProviderType { return aiproviders.ProviderAnthropic }

func (p Provider) ValidateCredentials(ctx context.Context, req aiproviders.CredentialRequest) error {
	key := apiKey(req.Credentials, req.Config)
	if key == "" {
		return fmt.Errorf("anthropic api key missing (set secret_ref to an OpenBao secret with api_key, or ANTHROPIC_API_KEY)")
	}
	baseURL := baseURL(req.Config, "https://api.anthropic.com")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/models", nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", anthropicVersion(req.Config))
	resp, err := p.http().Do(httpReq)
	if err != nil {
		return fmt.Errorf("anthropic credential check failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("anthropic credential check returned %s", resp.Status)
	}
	return nil
}

func (Provider) ListAvailableServices(context.Context, aiproviders.ServiceListRequest) ([]aiproviders.Service, error) {
	return []aiproviders.Service{
		{
			Name:                  "Claude API Access",
			Slug:                  "claude-api-access",
			Category:              "api_access",
			Description:           "Governed access to Anthropic Claude API usage.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "models", "expires_at"),
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "Claude Code Access",
			Slug:                  "claude-code-access",
			Category:              "developer_tool",
			Description:           "Governed Claude Code entitlement/license request with owner, expiry, and audit trail.",
			RequestSchema:         requestSchema("owner", "workspace", "justification", "repo_scope", "expires_at"),
			DefaultExpirationDays: 90,
			RequiresApproval:      true,
		},
	}, nil
}

// ListModels pulls the LIVE model catalog from Anthropic's /v1/models (the same
// endpoint ValidateCredentials probes), so a sync reflects the account's real
// models instead of a hardcoded subset.
func (p Provider) ListModels(ctx context.Context, req aiproviders.ModelListRequest) ([]aiproviders.Model, error) {
	key := apiKey(req.Credentials, req.Config)
	if key == "" {
		return nil, fmt.Errorf("anthropic api key missing (set secret_ref to an OpenBao secret with api_key, or ANTHROPIC_API_KEY)")
	}
	base := baseURL(req.Config, "https://api.anthropic.com")
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/v1/models?limit=1000", nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", anthropicVersion(req.Config))
	resp, err := p.http().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic model sync failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("anthropic model sync returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var payload struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
			CreatedAt   string `json:"created_at"`
			Type        string `json:"type"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decoding anthropic model list: %w", err)
	}
	models := make([]aiproviders.Model, 0, len(payload.Data))
	for _, item := range payload.Data {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		display := item.DisplayName
		if display == "" {
			display = item.ID
		}
		models = append(models, aiproviders.Model{
			Model:       item.ID,
			DisplayName: display,
			Modality:    "text",
			Status:      "active",
			Metadata:    map[string]any{"created_at": item.CreatedAt, "source": "anthropic_models_api"},
		})
	}
	return models, nil
}

func (Provider) ProvisionAccess(_ context.Context, req aiproviders.ProvisionRequest) (*aiproviders.ProvisionResult, error) {
	accessID := deterministicID("anthropic", req.RequestID.String(), req.Service.Slug, req.Owner)
	return &aiproviders.ProvisionResult{
		ProviderAccessID: accessID,
		Observed: map[string]any{
			"provider":              "anthropic",
			"provider_access_id":    accessID,
			"service":               req.Service.Slug,
			"owner":                 req.Owner,
			"workspace":             req.Workspace,
			"external_provisioning": "manual",
			"message":               "Claude/Claude Code access approved in OPORD; grant/revoke in Anthropic admin or existing IdP workflow.",
		},
	}, nil
}

func (Provider) RevokeAccess(_ context.Context, req aiproviders.RevokeRequest) error {
	if req.ProviderAccessID == "" {
		return fmt.Errorf("provider access id is required")
	}
	return nil
}

func (Provider) GetUsage(_ context.Context, req aiproviders.UsageRequest) ([]aiproviders.UsageRecord, error) {
	return []aiproviders.UsageRecord{
		{
			Metric:   "tokens",
			Quantity: 0,
			Unit:     "tokens",
			CostUSD:  0,
			Raw: map[string]any{
				"provider_access_id": req.ProviderAccessID,
				"source":             "not_imported",
				"message":            "Anthropic usage import is not implemented in this phase.",
			},
		},
	}, nil
}

func (Provider) GetStatus(_ context.Context, req aiproviders.StatusRequest) (*aiproviders.StatusResult, error) {
	return &aiproviders.StatusResult{
		Status: "active",
		Observed: map[string]any{
			"provider_access_id":    req.ProviderAccessID,
			"external_provisioning": "manual",
		},
	}, nil
}

func (p Provider) http() *http.Client {
	if p.client != nil {
		return p.client
	}
	return http.DefaultClient
}

func apiKey(creds map[string]string, _ map[string]any) string {
	for _, key := range []string{"api_key", "anthropic_api_key", "token"} {
		if v := strings.TrimSpace(creds[key]); v != "" {
			return v
		}
	}
	return strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
}

func baseURL(cfg map[string]any, fallback string) string {
	if v, ok := cfg["base_url"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func anthropicVersion(cfg map[string]any) string {
	if v, ok := cfg["anthropic_version"].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return "2023-06-01"
}

func deterministicID(parts ...string) string {
	sum := sha1.Sum([]byte(strings.Join(parts, ":")))
	return parts[0] + "-" + hex.EncodeToString(sum[:])[:16]
}

func requestSchema(fields ...string) map[string]any {
	return map[string]any{"fields": fields}
}
