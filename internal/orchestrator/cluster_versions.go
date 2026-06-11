package orchestrator

import (
	"context"
	"fmt"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/models"
	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/providers"
)

// ListProviderClusterVersions lists the managed-k8s versions a provider's cloud
// currently offers (GKE/AKS/EKS), so the cluster form can show a live picker
// instead of free-text (which let users type non-existent versions). Region
// defaults to the provider's configured region. Providers without a managed-k8s
// version API (vSphere/Proxmox, or a not-yet-implemented cloud) return a clear
// capability error and the UI falls back to "(provider default)" + free-text.
func (s *Service) ListProviderClusterVersions(ctx context.Context, providerName, region string) ([]string, error) {
	p, err := s.q.GetProviderByName(ctx, providerName)
	if err != nil {
		return nil, fmt.Errorf("provider %q not found: %w", providerName, err)
	}
	prov, err := s.registry.Get(models.ProviderType(p.Type))
	if err != nil {
		return nil, err
	}
	lister, ok := prov.(providers.ClusterVersionLister)
	if !ok {
		return nil, fmt.Errorf("provider %q (%s) does not support cluster version listing", providerName, p.Type)
	}
	cfg := s.providerCfg(ctx, p)
	if region == "" {
		if r, ok := cfg["region"].(string); ok {
			region = r
		}
	}
	creds, _ := s.creds.Resolve(ctx, p)
	return lister.ListClusterVersions(ctx, providers.ClusterVersionRequest{
		Region:      region,
		Credentials: creds,
		Config:      cfg,
	})
}
