// Package mock implements OPORD's MVP AI provider. It never calls external AI
// services; it deterministically grants and revokes mock access records.
package mock

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/aiproviders"
)

// Provider is the MockAIProvider implementation.
type Provider struct{}

// Register wires MockAIProvider into the AI provider registry.
func Register(r *aiproviders.Registry) {
	r.Register(aiproviders.ProviderMockAI, func() aiproviders.Provider { return Provider{} })
}

func (Provider) Type() aiproviders.ProviderType { return aiproviders.ProviderMockAI }

func (Provider) ValidateCredentials(context.Context, aiproviders.CredentialRequest) error {
	return nil
}

func (Provider) ListAvailableServices(context.Context, aiproviders.ServiceListRequest) ([]aiproviders.Service, error) {
	return []aiproviders.Service{
		{
			Name:                  "OpenAI API Access (Mock)",
			Slug:                  "openai-api-mock",
			Category:              "api_access",
			Description:           "Mock governed access to OpenAI-style API services for MVP testing.",
			RequestSchema:         map[string]any{"fields": []string{"owner", "workspace", "justification", "expires_at"}},
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "Claude Access (Mock)",
			Slug:                  "claude-access-mock",
			Category:              "api_access",
			Description:           "Mock governed access to Claude-style services for MVP testing.",
			RequestSchema:         map[string]any{"fields": []string{"owner", "workspace", "justification", "expires_at"}},
			DefaultExpirationDays: 30,
			RequiresApproval:      true,
		},
		{
			Name:                  "Kubernetes AI Sandbox (Mock)",
			Slug:                  "k8s-ai-sandbox-mock",
			Category:              "sandbox",
			Description:           "Mock AI sandbox entitlement; no cluster or external provider is created.",
			RequestSchema:         map[string]any{"fields": []string{"owner", "workspace", "justification", "expires_at"}},
			DefaultExpirationDays: 7,
			RequiresApproval:      true,
		},
	}, nil
}

func (Provider) ListModels(context.Context, aiproviders.ModelListRequest) ([]aiproviders.Model, error) {
	return []aiproviders.Model{
		{Model: "mock-gpt-4.1-mini", DisplayName: "Mock GPT-4.1 mini", Modality: "text", Status: "active", Metadata: map[string]any{"mock": true}},
		{Model: "mock-embedding-small", DisplayName: "Mock Embedding Small", Modality: "embedding", Status: "active", Metadata: map[string]any{"mock": true}},
		{Model: "mock-image-1", DisplayName: "Mock Image 1", Modality: "image", Status: "active", Metadata: map[string]any{"mock": true}},
	}, nil
}

func (Provider) ProvisionAccess(_ context.Context, req aiproviders.ProvisionRequest) (*aiproviders.ProvisionResult, error) {
	sum := sha1.Sum([]byte(req.RequestID.String() + ":" + req.Service.Slug + ":" + req.Owner))
	accessID := "mock-ai-" + hex.EncodeToString(sum[:])[:16]
	observed := map[string]any{
		"provider":           "mock_ai",
		"provider_access_id": accessID,
		"service":            req.Service.Slug,
		"owner":              req.Owner,
		"workspace":          req.Workspace,
		"external":           false,
		"message":            "Mock AI access provisioned; no external provider call was made.",
	}
	if req.ExpiresAt != nil {
		observed["expires_at"] = req.ExpiresAt.Format(time.RFC3339)
	}
	return &aiproviders.ProvisionResult{ProviderAccessID: accessID, Observed: observed}, nil
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
			Quantity: 12500,
			Unit:     "tokens",
			CostUSD:  1.25,
			Raw: map[string]any{
				"provider_access_id": req.ProviderAccessID,
				"mock":               true,
			},
		},
	}, nil
}

func (Provider) GetStatus(_ context.Context, req aiproviders.StatusRequest) (*aiproviders.StatusResult, error) {
	status := "active"
	if req.ProviderAccessID == "" {
		status = "unknown"
	}
	return &aiproviders.StatusResult{
		Status: status,
		Observed: map[string]any{
			"provider_access_id": req.ProviderAccessID,
			"mock":               true,
		},
	}, nil
}
