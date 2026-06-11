package orchestrator

import (
	"context"
	"fmt"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// ListProviderImages lists bootable images (e.g. AWS AMIs) for a provider that
// supports it, so the UI can offer a picker instead of free-text IDs. Region
// defaults to the provider's configured region. Providers without an image
// catalog (vSphere/Proxmox) return a clear capability error.
func (s *Service) ListProviderImages(ctx context.Context, providerName, region, owner string) ([]providers.Image, error) {
	p, err := s.q.GetProviderByName(ctx, providerName)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found: %w", providerName, err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return nil, err
	}
	lister, ok := prov.(providers.ImageLister)
	if !ok {
		return nil, fmt.Errorf("provider %q (%s) does not support image listing", providerName, p.Type)
	}

	cfg := s.providerCfg(ctx, p)
	if region == "" {
		if r, ok := cfg["region"].(string); ok {
			region = r
		}
	}
	creds, _ := s.creds.Resolve(ctx, p)
	return lister.ListImages(ctx, providers.ImageRequest{
		Region:      region,
		Owner:       owner,
		Credentials: creds,
		Config:      cfg,
	})
}
