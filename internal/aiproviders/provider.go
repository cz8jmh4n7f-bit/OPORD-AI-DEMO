// Package aiproviders defines provider-neutral AI service access operations.
// It intentionally stays separate from internal/providers, whose interfaces are
// infrastructure/OpenTofu oriented.
package aiproviders

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// ProviderType identifies an AI service backend.
type ProviderType string

const (
	ProviderMockAI        ProviderType = "mock_ai"
	ProviderOpenAI        ProviderType = "openai"
	ProviderAnthropic     ProviderType = "anthropic"
	ProviderGemini        ProviderType = "gemini"
	ProviderGitHubCopilot ProviderType = "github_copilot"
	ProviderCursor        ProviderType = "cursor"
	ProviderLiteLLM       ProviderType = "litellm"
)

// Service is a provider-advertised AI catalog entry.
type Service struct {
	Name                  string
	Slug                  string
	Category              string
	Description           string
	RequestSchema         map[string]any
	DefaultExpirationDays int
	RequiresApproval      bool
}

// Model is a provider-visible model catalog entry. It is governance metadata,
// not a provisioning primitive.
type Model struct {
	Model       string
	DisplayName string
	Modality    string
	Status      string
	Metadata    map[string]any
}

// CredentialRequest checks resolved credentials and non-secret config.
type CredentialRequest struct {
	Credentials map[string]string
	Config      map[string]any
}

// ServiceListRequest carries resolved credentials/config for service discovery.
type ServiceListRequest struct {
	Credentials map[string]string
	Config      map[string]any
}

// ModelListRequest carries resolved credentials/config for model discovery.
type ModelListRequest struct {
	Credentials map[string]string
	Config      map[string]any
}

// ProvisionRequest carries an approved AI access request.
type ProvisionRequest struct {
	RequestID   uuid.UUID
	ServiceID   uuid.UUID
	Service     Service
	Owner       string
	Workspace   string
	Spec        json.RawMessage
	ExpiresAt   *time.Time
	Credentials map[string]string
	Config      map[string]any
}

// ProvisionResult is the provider-side result of granting access.
type ProvisionResult struct {
	ProviderAccessID string
	Observed         map[string]any
}

// RevokeRequest carries the provider access identifier to revoke.
type RevokeRequest struct {
	InstanceID       uuid.UUID
	ProviderAccessID string
	Credentials      map[string]string
	Config           map[string]any
}

// UsageRequest asks a provider for usage over a period.
type UsageRequest struct {
	ProviderAccessID string
	PeriodStart      time.Time
	PeriodEnd        time.Time
	Credentials      map[string]string
	Config           map[string]any
}

// UsageRecord is one provider usage datum.
type UsageRecord struct {
	Metric   string
	Quantity float64
	Unit     string
	CostUSD  float64
	Raw      map[string]any
}

// StatusRequest asks a provider for the current access status.
type StatusRequest struct {
	ProviderAccessID string
	Credentials      map[string]string
	Config           map[string]any
}

// StatusResult is a provider status response.
type StatusResult struct {
	Status   string
	Observed map[string]any
}

// Provider abstracts an AI backend (MockAI, OpenAI, Anthropic, ...).
type Provider interface {
	Type() ProviderType
	ValidateCredentials(ctx context.Context, req CredentialRequest) error
	ListAvailableServices(ctx context.Context, req ServiceListRequest) ([]Service, error)
	ProvisionAccess(ctx context.Context, req ProvisionRequest) (*ProvisionResult, error)
	RevokeAccess(ctx context.Context, req RevokeRequest) error
	GetUsage(ctx context.Context, req UsageRequest) ([]UsageRecord, error)
	GetStatus(ctx context.Context, req StatusRequest) (*StatusResult, error)
}

// ModelCatalogProvider is implemented by providers that can expose a model
// catalog. It stays optional so the base access lifecycle interface remains
// stable for providers that only govern seats or tools.
type ModelCatalogProvider interface {
	ListModels(ctx context.Context, req ModelListRequest) ([]Model, error)
}

// Factory builds a Provider.
type Factory func() Provider

// Registry maps AI provider types to factories.
type Registry struct {
	factories map[ProviderType]Factory
}

// NewRegistry returns an empty AI provider registry.
func NewRegistry() *Registry {
	return &Registry{factories: make(map[ProviderType]Factory)}
}

// Register associates a provider type with a factory.
func (r *Registry) Register(t ProviderType, f Factory) {
	if _, exists := r.factories[t]; exists {
		panic(fmt.Sprintf("aiproviders: type %q already registered", t))
	}
	r.factories[t] = f
}

// Get instantiates the provider for the given type.
func (r *Registry) Get(t ProviderType) (Provider, error) {
	f, ok := r.factories[t]
	if !ok {
		return nil, fmt.Errorf("aiproviders: no provider registered for type %q", t)
	}
	return f(), nil
}

// Types returns the registered provider types in sorted order.
func (r *Registry) Types() []ProviderType {
	types := make([]ProviderType, 0, len(r.factories))
	for t := range r.factories {
		types = append(types, t)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })
	return types
}
