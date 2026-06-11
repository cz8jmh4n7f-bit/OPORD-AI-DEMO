package orchestrator

import (
	"context"
	"fmt"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// ListProviderBillingScopes lists the billing scopes (Azure MCA invoice sections)
// a provider's credentials can create subscriptions under, so the account form
// can offer a picker instead of a pasted resource-id URI. Providers without a
// billing API (AWS/GCP/vSphere/Proxmox) return a clear capability error and the
// frontend falls back to manual entry.
func (s *Service) ListProviderBillingScopes(ctx context.Context, providerName string) ([]providers.BillingScope, error) {
	p, err := s.q.GetProviderByName(ctx, providerName)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found: %w", providerName, err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return nil, err
	}
	lister, ok := prov.(providers.BillingScopeLister)
	if !ok {
		return nil, fmt.Errorf("provider %q (%s) does not support billing-scope listing", providerName, p.Type)
	}

	cfg := s.providerCfg(ctx, p)
	creds, _ := s.creds.Resolve(ctx, p)
	return lister.ListBillingScopes(ctx, providers.BillingScopeRequest{
		Credentials: creds,
		Config:      cfg,
	})
}
