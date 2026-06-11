package orchestrator

import (
	"context"

	"github.com/cz8jmh4n7f-bit/opord-ai-demo/internal/db"
)

// Provider returns a registered provider by name.
func (s *Service) Provider(ctx context.Context, name string) (db.Provider, error) {
	return s.q.GetProviderByName(ctx, name)
}
